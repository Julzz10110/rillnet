package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	schemaVersionKey = "rillnet:schema:version"
	currentSchemaVersion = 1
)

// Migration represents a database migration
type Migration struct {
	Version int
	Up      func(ctx context.Context, client *redis.Client) error
	Down    func(ctx context.Context, client *redis.Client) error
}

// Migrate runs all pending migrations
func Migrate(ctx context.Context, client *redis.Client, logger *zap.SugaredLogger) error {
	// Get current schema version
	currentVersion, err := getSchemaVersion(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	if currentVersion >= currentSchemaVersion {
		if logger != nil {
			logger.Infow("schema is up to date",
				"current_version", currentVersion,
				"target_version", currentSchemaVersion,
			)
		}
		return nil
	}

	// Run migrations
	migrations := getMigrations()
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			if logger != nil {
				logger.Infow("running migration",
					"version", migration.Version,
				)
			}

			if err := migration.Up(ctx, client); err != nil {
				return fmt.Errorf("migration %d failed: %w", migration.Version, err)
			}

			// Update schema version
			if err := setSchemaVersion(ctx, client, migration.Version); err != nil {
				return fmt.Errorf("failed to update schema version: %w", err)
			}

			if logger != nil {
				logger.Infow("migration completed",
					"version", migration.Version,
				)
			}
		}
	}

	// Set final version
	if err := setSchemaVersion(ctx, client, currentSchemaVersion); err != nil {
		return fmt.Errorf("failed to set final schema version: %w", err)
	}

	if logger != nil {
		logger.Infow("all migrations completed",
			"final_version", currentSchemaVersion,
		)
	}

	return nil
}

// getSchemaVersion gets the current schema version from Redis
func getSchemaVersion(ctx context.Context, client *redis.Client) (int, error) {
	val, err := client.Get(ctx, schemaVersionKey).Int()
	if err == redis.Nil {
		return 0, nil // No version set, start from 0
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}

// setSchemaVersion sets the schema version in Redis
func setSchemaVersion(ctx context.Context, client *redis.Client, version int) error {
	return client.Set(ctx, schemaVersionKey, version, 0).Err()
}

// getMigrations returns all migrations in order
func getMigrations() []Migration {
	return []Migration{
		{
			Version: 1,
			Up: func(ctx context.Context, client *redis.Client) error {
				// Migration 1: Initialize schema
				// This migration ensures all required keys and structures exist
				// Since we're using simple key-value storage, this is mainly for versioning
				
				// Create index for active streams if it doesn't exist
				activeKey := "rillnet:stream:active"
				exists, err := client.Exists(ctx, activeKey).Result()
				if err != nil {
					return err
				}
				if exists == 0 {
					// Create empty set
					if err := client.SAdd(ctx, activeKey, "").Err(); err != nil {
						return err
					}
					// Remove placeholder
					client.SRem(ctx, activeKey, "")
				}

				return nil
			},
			Down: func(ctx context.Context, client *redis.Client) error {
				// Rollback migration 1
				// In a production system, this would clean up created structures
				return nil
			},
		},
	}
}

