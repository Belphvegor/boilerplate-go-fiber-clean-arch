package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/casper/go-fiber-clean-arch/config"
)

// Connection represents a unified database handle for the application.
type Connection struct {
	SQL   *sqlx.DB
	Mongo *mongo.Database
}

// Close gracefully shuts down any active connections.
func (c *Connection) Close(ctx context.Context) error {
	var err error
	if c.SQL != nil {
		err = c.SQL.Close()
	}
	if c.Mongo != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if disconnectErr := c.Mongo.Client().Disconnect(ctx); disconnectErr != nil && err == nil {
			err = disconnectErr
		}
	}
	return err
}

// Factory produces database connections according to configuration.
type Factory struct {
	cfg *config.AppConfig
}

// NewFactory creates a database factory with the provided configuration.
func NewFactory(cfg *config.AppConfig) *Factory {
	return &Factory{cfg: cfg}
}

// Open establishes the configured database connection.
func (f *Factory) Open(ctx context.Context) (*Connection, error) {
	switch f.cfg.Database.Driver {
	case "postgres", "mysql":
		db, err := sqlx.Open(f.cfg.Database.Driver, f.cfg.Database.DSN)
		if err != nil {
			return nil, fmt.Errorf("open sql connection: %w", err)
		}

		db.SetMaxOpenConns(f.cfg.Database.MaxOpenConns)
		db.SetMaxIdleConns(f.cfg.Database.MaxIdleConns)
		db.SetConnMaxLifetime(f.cfg.Database.ConnMaxLife)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("ping sql connection: %w", err)
		}

		return &Connection{SQL: db}, nil

	case "mongo":
		clientOpts := options.Client().ApplyURI(f.cfg.Mongo.URI)
		client, err := mongo.Connect(ctx, clientOpts)
		if err != nil {
			return nil, fmt.Errorf("connect mongo: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := client.Ping(ctx, readpref.Primary()); err != nil {
			return nil, fmt.Errorf("ping mongo: %w", err)
		}

		db := client.Database(f.cfg.Mongo.Database)
		return &Connection{Mongo: db}, nil

	default:
		return nil, fmt.Errorf("unknown database driver %q", f.cfg.Database.Driver)
	}
}
