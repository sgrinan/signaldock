package endpoint

import (
	"context"
	"errors"
	"fmt"

	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists endpoint configuration in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository returns a Repository backed by pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// Insert persists an endpoint.
func (r *Repository) Insert(endpoint Endpoint) error {
	const query = `
		INSERT INTO endpoints (id, url)
		VALUES ($1, $2)
	`

	_, err := r.pool.Exec(context.Background(), query, endpoint.ID, endpoint.URL)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEndpointExists
		}

		return fmt.Errorf("insert endpoint: %w", err)
	}

	return nil
}

// List returns all persisted endpoints.
func (r *Repository) List() ([]Endpoint, error) {
	const query = `
		SELECT id::text, url
		FROM endpoints
		ORDER BY created_at, id
	`

	rows, err := r.pool.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []Endpoint

	for rows.Next() {
		var (
			endpoint Endpoint
			id       string
		)

		if err := rows.Scan(&id, &endpoint.URL); err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}

		endpoint.ID, err = uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("parse endpoint id: %w", err)
		}

		endpoints = append(endpoints, endpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate endpoints: %w", err)
	}

	return endpoints, nil
}

// ByID returns the persisted endpoint with the given ID.
func (r *Repository) ByID(id uuid.UUID) (Endpoint, error) {
	const query = `
		SELECT id::text, url
		FROM endpoints
		WHERE id = $1
	`

	var (
		endpoint Endpoint
		rawID    string
	)

	err := r.pool.QueryRow(context.Background(), query, id).Scan(&rawID, &endpoint.URL)

	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrEndpointNotFound
	}

	if err != nil {
		return Endpoint{}, fmt.Errorf("get endpoint: %w", err)
	}

	endpoint.ID, err = uuid.Parse(rawID)
	if err != nil {
		return Endpoint{}, fmt.Errorf("parse endpoint id: %w", err)
	}

	return endpoint, nil
}

// RemoveByID removes the persisted endpoint with the given ID.
func (r *Repository) RemoveByID(id uuid.UUID) error {
	const query = `
		DELETE FROM endpoints
		WHERE id = $1
	`

	result, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("remove endpoint: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrEndpointNotFound
	}

	return nil
}
