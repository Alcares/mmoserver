package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"log/slog"
	"net/url"

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
	return conn, nil
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
	if reason := validateUsername(username); reason != pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED {
		return nil, reason
	}
	if reason := validatePassword(password); reason != pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED {
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

func validatePassword(password string) pb.AccountCreateRejection {
	return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
}

func validateUsername(username string) pb.AccountCreateRejection {
	return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
}
