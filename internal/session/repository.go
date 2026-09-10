package session

import (
	"context"
	"errors"
	"fmt"

	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists sessions in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository returns a Repository backed by pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) Insert(session Session) error {
	const query = `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`

	_, err := r.pool.Exec(context.Background(), query, session.TokenHash, session.UserID, session.ExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSessionExists
		}

		return fmt.Errorf("insert session: %w", err)
	}

	return nil
}

func (r *Repository) ByTokenHash(tokenHash string) (Session, error) {
	const query = `
		SELECT token_hash, user_id::text, expires_at, created_at
		FROM sessions
		WHERE token_hash = $1
	`

	var (
		session Session
		rawID   string
	)

	err := r.pool.QueryRow(context.Background(), query, tokenHash).Scan(&session.TokenHash, &rawID, &session.ExpiresAt, &session.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}

	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	session.UserID, err = uuid.Parse(rawID)
	if err != nil {
		return Session{}, fmt.Errorf("parse session user id: %w", err)
	}

	return session, nil
}

func (r *Repository) Delete(tokenHash string) error {
	const query = `
		DELETE FROM sessions
		WHERE token_hash = $1
	`

	result, err := r.pool.Exec(context.Background(), query, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (r *Repository) DeleteExpired() error {
	const query = `
		DELETE FROM sessions
		WHERE expires_at <= NOW()
	`

	_, err := r.pool.Exec(context.Background(), query)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}

	return nil
}
