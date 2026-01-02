package database

import (
	"context"
	"fmt"
	"time"

	"github.com/BrunoTulio/logr"
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
	log    logr.Logger
}

func NewClient(cfg *config.DatabaseConfig, log logr.Logger) *Client {
	return &Client{config: cfg, log: log}
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

func (c *Client) DropDatabase(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, c.config.AdminConnectionString())
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()
	dbName := c.config.Name
	if dbName == "postgres" {
		return fmt.Errorf("refusing to drop system database 'postgres'")
	}

	c.log.Warnf("⚠️  Dropping database '%s' (all data will be LOST)", dbName)

	terminateSQL := fmt.Sprintf(`
        DO $$
        BEGIN
            PERFORM pg_terminate_backend(pid)
            FROM pg_stat_activity
            WHERE datname = '%s' AND pid <> pg_backend_pid();
        END$$;`, dbName)

	if _, err := conn.Exec(ctx, terminateSQL); err != nil {
		return fmt.Errorf("failed to terminate connections on '%s': %w", dbName, err)
	}
	c.log.Debugf("✅ Terminated connections on '%s'", dbName)

	// 2. DROP DATABASE (commit automático após Exec)
	dropSQL := fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)
	if _, err := conn.Exec(ctx, dropSQL); err != nil {
		return fmt.Errorf("failed to drop database '%s': %w", dbName, err)
	}
	c.log.Debugf("✅ Dropped database '%s'", dbName)

	// 3. CREATE DATABASE novo (commit automático após Exec)
	createSQL := fmt.Sprintf(`
        CREATE DATABASE "%s"
        OWNER "%s"
        ENCODING 'UTF8'
        LC_COLLATE 'C'
        LC_CTYPE 'C'
        TEMPLATE template0;`, dbName, c.config.Username)

	if _, err := conn.Exec(ctx, createSQL); err != nil {
		return fmt.Errorf("failed to create database '%s': %w", dbName, err)
	}

	c.log.Infof("✅ Database '%s' dropped and recreated successfully", dbName)
	return nil
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
