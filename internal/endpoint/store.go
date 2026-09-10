package endpoint

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists endpoint configuration in PostgreSQL and keeps
// the latest check results in memory.
type Store struct {
	pool *pgxpool.Pool

	mu         sync.RWMutex
	lastChecks map[uuid.UUID]CheckResult
}

// NewStore returns a Store backed by pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:       pool,
		lastChecks: make(map[uuid.UUID]CheckResult),
	}
}

func (s *Store) Insert(endpoint Endpoint) error {
	const query = `
		INSERT INTO endpoints (id, url)
		VALUES ($1, $2)
	`

	_, err := s.pool.Exec(context.Background(), query, endpoint.ID, endpoint.URL)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEndpointExists
		}

		return fmt.Errorf("insert endpoint: %w", err)
	}

	s.mu.Lock()
	s.lastChecks[endpoint.ID] = endpoint.LastCheck
	s.mu.Unlock()

	return nil
}

func (s *Store) List() ([]Endpoint, error) {
	const query = `
		SELECT id::text, url
		FROM endpoints
		ORDER BY created_at, id
	`

	rows, err := s.pool.Query(context.Background(), query)
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

		s.mu.RLock()
		endpoint.LastCheck = s.lastChecks[endpoint.ID]
		s.mu.RUnlock()

		endpoints = append(endpoints, endpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate endpoints: %w", err)
	}

	return endpoints, nil
}

func (s *Store) ByID(id uuid.UUID) (Endpoint, error) {
	const query = `
		SELECT id::text, url
		FROM endpoints
		WHERE id = $1
	`

	var (
		endpoint Endpoint
		rawID    string
	)

	err := s.pool.QueryRow(context.Background(), query, id).Scan(&rawID, &endpoint.URL)

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

	s.mu.RLock()
	endpoint.LastCheck = s.lastChecks[endpoint.ID]
	s.mu.RUnlock()

	return endpoint, nil
}

func (s *Store) RemoveByID(id uuid.UUID) error {
	const query = `
		DELETE FROM endpoints
		WHERE id = $1
	`

	result, err := s.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("remove endpoint: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrEndpointNotFound
	}

	s.mu.Lock()
	delete(s.lastChecks, id)
	s.mu.Unlock()

	return nil
}

func (s *Store) UpdateLastCheck(id uuid.UUID, result CheckResult) error {
	const query = `
		SELECT 1
		FROM endpoints
		WHERE id = $1
	`

	var exists int

	err := s.pool.QueryRow(context.Background(), query, id).Scan(&exists)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEndpointNotFound
	}

	if err != nil {
		return fmt.Errorf("check endpoint existence: %w", err)
	}

	s.mu.Lock()
	s.lastChecks[id] = result
	s.mu.Unlock()

	return nil
}
