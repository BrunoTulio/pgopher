package database

import (
	"context"
	"fmt"

	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/jackc/pgx/v5"
)

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
