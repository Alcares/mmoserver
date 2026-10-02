package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/alcares/mmoserver/backend/gen/go/db"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// dummyPasswordHash exist to make it impossible to tell if a username exists or not by timing
const dummyPasswordHash = "$2a$10$D4Y.N8T8nCu2STIhLuDHy.hbA73FOpjnF7LQjRI/rvyZr2Bx.IRBy"

// Open creates a single connection to the SQLite DB. Caller has to close.
func Open(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")  // readers don't block the writer
	q.Add("_pragma", "busy_timeout(5000)") // a blocked write waits up to 5 seconds instead of failing immediately
	q.Add("_pragma", "foreign_keys(1)")    // SQLite doesn't enforce foreign keys unless this is on
	q.Set("_txlock", "immediate")          // BeginTx takes the write lock right away. This avoids the lock-upgrade deadlock possibility

	conn, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil { // sql.Open is lazy; this actually opens the file
		conn.Close()
		return nil, err
	}

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return conn, nil
}

//go:embed migrations/*.sql
var migrations embed.FS

// migrate applies every migration newer than the database's user_version, each in its own transaction
func migrate(conn *sql.DB) error {
	var version int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}

	files, err := fs.Glob(migrations, "migrations/*.sql") // sorted by name
	if err != nil {
		return err
	}

	for _, file := range files {
		n, err := strconv.Atoi(strings.SplitN(path.Base(file), "_", 2)[0])
		if err != nil {
			return fmt.Errorf("%s: name must start with a number", file)
		}
		if n <= version {
			continue
		}

		body, err := migrations.ReadFile(file)
		if err != nil {
			return err
		}

		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", file, err)
		}
		// PRAGMA takes no ? parameters; n is an int parsed above, so Sprintf is safe
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", n)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		slog.Info("applied migration", "file", file)
		version = n
	}
	return nil
}

// Accounts creates and authenticates player accounts
type Accounts struct {
	conn    *sql.DB
	queries *db.Queries
}

func NewAccounts(conn *sql.DB) *Accounts {
	return &Accounts{conn: conn, queries: db.New(conn)}
}

// Session identifies the account behind a connection
type Session struct {
	AccountID uuid.UUID
	Name      string
	Rating    float64
}

// ApplyRatingChanges adds each account's rating change, all or none
func (a *Accounts) ApplyRatingChanges(ctx context.Context, changes map[uuid.UUID]float64) error {
	tx, err := a.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit

	q := a.queries.WithTx(tx)
	for id, delta := range changes {
		if _, err = q.UpdateRating(ctx, db.UpdateRatingParams{Rating: delta, ID: id[:]}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ErrInvalidCredentials means the username doesn't exist or the password is wrong; the two are
// deliberately indistinguishable
var ErrInvalidCredentials = errors.New("invalid username or password")

// ErrUsernameTaken means another account already has the name, ignoring case
var ErrUsernameTaken = errors.New("username taken")

func (a *Accounts) Login(ctx context.Context, username, password string) (*Session, error) {
	account, err := a.queries.GetAccount(ctx, username)
	switch {
	case errors.Is(err, sql.ErrNoRows), err == nil && account.Name != username:
		// The lookup ignores case, but a login has to match the name exactly
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		return nil, ErrInvalidCredentials // no such user
	case err != nil:
		return nil, fmt.Errorf("load account: %w", err)
	}

	id, err := uuid.FromBytes(account.ID)
	if err != nil {
		return nil, fmt.Errorf("parse account id: %w", err)
	}

	err = bcrypt.CompareHashAndPassword(account.PasswordHash, []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := a.queries.TouchLastSeen(ctx, account.ID); err != nil {
		slog.Warn("update last_seen", "err", err) // don't fail the login over this
	}

	slog.Info("logged in", "username", username)
	return &Session{
		AccountID: id,
		Name:      account.Name,
		Rating:    account.Rating,
	}, nil
}

// CreateNewAccount stores a new account and logs it in; the caller validates username and password
func (a *Accounts) CreateNewAccount(ctx context.Context, username, password string) (*Session, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate account id: %w", err)
	}

	account, err := a.queries.CreateAccount(ctx, db.CreateAccountParams{
		ID:           id[:],
		Name:         username,
		PasswordHash: hash,
	})

	var sqlErr *sqlite.Error
	switch {
	case errors.As(err, &sqlErr) && sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE:
		return nil, ErrUsernameTaken
	case err != nil:
		return nil, fmt.Errorf("insert account: %w", err)
	}

	id, err = uuid.FromBytes(account.ID)
	if err != nil {
		return nil, fmt.Errorf("parse account id: %w", err)
	}

	slog.Info("account created", "id", id)

	return &Session{
		AccountID: id,
		Name:      account.Name,
		Rating:    account.Rating,
	}, nil
}
