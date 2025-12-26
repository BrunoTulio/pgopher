package database

import (
	"context"
	"fmt"
	"time"

	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/jackc/pgx/v5"
)

type ConnectionInfo struct {
	PID        int
	Username   string
	AppName    string
	ClientAddr string
	State      string
	QueryStart time.Time
}
type Client struct {
	config *config.DatabaseConfig
}

func NewClient(cfg *config.DatabaseConfig) *Client {
	return &Client{config: cfg}
}

func (c *Client) TestConnection(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, c.config.ConnectionString())
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()
	if err := conn.Ping(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetSize(ctx context.Context) (int64, error) {
	conn, err := pgx.Connect(ctx, c.config.ConnectionString())
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	var size int64
	query := fmt.Sprintf("SELECT pg_database_size('%s')", c.config.Name)
	err = conn.QueryRow(ctx, query).Scan(&size)
	if err != nil {
		return 0, err
	}

	return size, nil
}

func (c *Client) GetVersion(ctx context.Context) (string, error) {
	conn, err := pgx.Connect(ctx, c.config.ConnectionString())
	if err != nil {
		return "", err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	var version string
	err = conn.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", err
	}

	return version, nil
}

func (c *Client) Ping(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, c.config.ConnectionString())
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	return conn.Ping(ctx)
}

func (c *Client) CountConnections(ctx context.Context) (int, error) {

	conn, err := pgx.Connect(ctx, c.config.ConnectionString())
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	var count int
	err = conn.QueryRow(ctx, fmt.Sprintf(`
        SELECT COUNT(*) 
        FROM pg_stat_activity 
        WHERE datname = '%s' 
        AND pid <> pg_backend_pid()
    `, c.config.Name)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to check connections: %w", err)

	}

	return count, nil

}

func (c *Client) ListConnections(ctx context.Context) ([]ConnectionInfo, error) {
	conn, err := pgx.Connect(ctx, c.config.ConnectionString())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()
	query := fmt.Sprintf(`
        SELECT 
            pid,
            usename,
            COALESCE(application_name, 'unknown'),
            COALESCE(client_addr::text, 'local'),
            state,
            query_start
        FROM pg_stat_activity 
        WHERE datname = '%s' 
        AND pid <> pg_backend_pid()
        ORDER BY query_start DESC
        LIMIT 10
    `, c.config.Name)

	rows, err := conn.Query(ctx, query)

	if err != nil {
		return nil, fmt.Errorf("failed to list connections: %w", err)
	}

	defer rows.Close()

	var connections []ConnectionInfo
	for rows.Next() {
		var conn ConnectionInfo
		if err := rows.Scan(
			&conn.PID,
			&conn.Username,
			&conn.AppName,
			&conn.ClientAddr,
			&conn.State,
			&conn.QueryStart,
		); err != nil {
			continue
		}
		connections = append(connections, conn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list connections: %w", err)
	}

	return connections, nil
}
