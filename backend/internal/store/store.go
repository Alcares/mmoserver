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
	"unicode"
	"unicode/utf8"

	"github.com/alcares/mmoserver/backend/gen/go/db"
	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

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

//go:embed common_passwords.txt
var commonPasswordsFile string

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
	queries *db.Queries
}

func NewAccounts(conn *sql.DB) *Accounts {
	return &Accounts{queries: db.New(conn)}
}

// Session identifies the account behind a connection
type Session struct {
	AccountID uuid.UUID
	Name      string
}

func (a *Accounts) Login(ctx context.Context, username, password string) (*Session, pb.LoginRejection) {
	account, err := a.queries.GetAccount(ctx, username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, pb.LoginRejection_LOGIN_REJECTION_INVALID_CREDENTIALS // no such user
	case err != nil:
		slog.Error("load account", "err", err)
		return nil, pb.LoginRejection_LOGIN_REJECTION_SERVER_ERROR // DB broken, timeout, ...
	}

	id, err := uuid.FromBytes(account.ID)
	if err != nil {
		slog.Error("parse account id", "err", err)
		return nil, pb.LoginRejection_LOGIN_REJECTION_SERVER_ERROR
	}

	err = bcrypt.CompareHashAndPassword(account.PasswordHash, []byte(password))
	if err != nil {
		return nil, pb.LoginRejection_LOGIN_REJECTION_INVALID_CREDENTIALS
	}

	if err := a.queries.TouchLastSeen(ctx, account.ID); err != nil {
		slog.Warn("update last_seen", "err", err) // don't fail the login over this
	}

	slog.Info("logged in", "username", username)
	return &Session{
		AccountID: id,
		Name:      account.Name,
	}, pb.LoginRejection_LOGIN_REJECTION_UNSPECIFIED
}

func (a *Accounts) CreateNewAccount(ctx context.Context, username, password string) (*Session, pb.AccountCreateRejection) {
	username = strings.TrimSpace(username)

	if reason := validateUsername(username); reason != pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED {
		return nil, reason
	}
	if reason := validatePassword(username, password); reason != pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED {
		return nil, reason
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("hash password", "err", err)
		return nil, pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_SERVER_ERROR
	}
	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("generate account id", "err", err)
		return nil, pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_SERVER_ERROR
	}

	account, err := a.queries.CreateAccount(ctx, db.CreateAccountParams{
		ID:           id[:],
		Name:         username,
		PasswordHash: hash,
	})

	var sqlErr *sqlite.Error
	switch {
	case errors.As(err, &sqlErr) && sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE:
		return nil, pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TAKEN
	case err != nil:
		slog.Error("insert account", "err", err)
		return nil, pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_SERVER_ERROR
	}

	id, err = uuid.FromBytes(account.ID)
	if err != nil {
		slog.Error("parse account id", "err", err)
		return nil, pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_SERVER_ERROR
	}

	slog.Info("account created", "id", id)

	return &Session{
		AccountID: id,
		Name:      account.Name,
	}, pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
}

const (
	passwordMinLength       = 8
	passwordMaxLength       = 72
	passwordMinNumbers      = 1
	passwordMinSpecialChars = 1

	usernameMinLength = 2
	usernameMaxLength = 20
)

// commonPasswords holds the embedded list, lowercased, built once at startup
var commonPasswords = func() map[string]struct{} {
	m := make(map[string]struct{})
	for line := range strings.SplitSeq(commonPasswordsFile, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			m[strings.ToLower(p)] = struct{}{}
		}
	}
	return m
}()

func validatePassword(username, password string) pb.AccountCreateRejection {
	switch {
	case utf8.RuneCountInString(password) < passwordMinLength:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_SHORT
	case len(password) > passwordMaxLength: // bcrypt's limit is 72 bytes
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_LONG
	}

	lower := strings.ToLower(password)
	if _, common := commonPasswords[lower]; common || strings.Contains(lower, strings.ToLower(username)) {
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_COMMON
	}

	var numbers, specialChars int
	r := []rune(password)
	for _, c := range r {
		if unicode.IsNumber(c) {
			numbers++
		}
		if unicode.IsSymbol(c) || unicode.IsPunct(c) {
			specialChars++
		}
	}

	switch {
	case numbers < passwordMinNumbers:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_MISSING_NUMBERS
	case specialChars < passwordMinSpecialChars:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_MISSING_SPECIAL_CHARACTERS
	}

	return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
}

func validateUsername(username string) pb.AccountCreateRejection {
	switch {
	case utf8.RuneCountInString(username) < usernameMinLength:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TOO_SHORT
	case utf8.RuneCountInString(username) > usernameMaxLength:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TOO_LONG
	}

	r := []rune(username)
	for _, c := range r {
		if !unicode.IsLetter(c) && !unicode.IsNumber(c) && c != ' ' {
			return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_INVALID_CHARACTERS
		}
	}

	return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
}
