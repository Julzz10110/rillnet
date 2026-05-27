package repositories

import (
	"context"
	"fmt"

	"rillnet/internal/core/ports"
	"rillnet/internal/infrastructure/repositories/memory"
	pgrepo "rillnet/internal/infrastructure/repositories/postgres"
	redisrepo "rillnet/internal/infrastructure/repositories/redis"
	"rillnet/pkg/config"

	"github.com/redis/go-redis/v9"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// RepositoryFactory creates repositories with fallback support
type RepositoryFactory struct {
	useRedis    bool
	redisClient *redis.Client
	useDB       bool
	dbPool      *pgxpool.Pool
	logger      *zap.SugaredLogger
}

func (f *RepositoryFactory) DBPool() *pgxpool.Pool {
	return f.dbPool
}

func (f *RepositoryFactory) CreateUserRepository() ports.UserRepository {
	if f.dbPool != nil {
		return pgrepo.NewUserRepository(f.dbPool)
	}
	return nil
}

func (f *RepositoryFactory) CreateRefreshTokenRepository() ports.RefreshTokenRepository {
	if f.dbPool != nil {
		return pgrepo.NewRefreshTokenRepository(f.dbPool)
	}
	return nil
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(cfg *config.Config, logger *zap.SugaredLogger) (*RepositoryFactory, error) {
	factory := &RepositoryFactory{
		useRedis: cfg.Redis.Enabled,
		useDB:    cfg.Database.Enabled,
		logger:   logger,
	}

	// Try to connect to Redis if enabled
	if cfg.Redis.Enabled {
		client, err := redisrepo.NewRedisClient(
			cfg.Redis.Address,
			cfg.Redis.Password,
			cfg.Redis.DB,
			cfg.Redis.PoolSize,
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("redis is enabled but connection failed: %w", err)
		} else {
			factory.redisClient = client
			logger.Info("using Redis repositories")
		}
	}

	if !factory.useRedis {
		logger.Info("using memory repositories")
	}

	if cfg.Database.Enabled {
		pool, err := pgrepo.NewPool(context.Background(), cfg.Database.DSN)
		if err != nil {
			return nil, fmt.Errorf("database is enabled but connection failed: %w", err)
		}
		factory.dbPool = pool
		logger.Info("using Postgres database")
	}

	return factory, nil
}

// CreatePeerRepository creates a peer repository (Redis or memory with fallback)
func (f *RepositoryFactory) CreatePeerRepository() ports.PeerRepository {
	if f.useRedis && f.redisClient != nil {
		return redisrepo.NewRedisPeerRepository(f.redisClient)
	}
	return memory.NewMemoryPeerRepository()
}

// CreateStreamRepository creates a stream repository (Redis or memory with fallback)
func (f *RepositoryFactory) CreateStreamRepository() ports.StreamRepository {
	if f.useRedis && f.redisClient != nil {
		return redisrepo.NewRedisStreamRepository(f.redisClient)
	}
	return memory.NewMemoryStreamRepository()
}

// CreateMeshRepository creates a mesh repository (always memory for now)
func (f *RepositoryFactory) CreateMeshRepository() ports.MeshRepository {
	// Mesh repository is always memory for now
	// Can be extended to Redis later if needed
	return memory.NewMemoryMeshRepository()
}

// Close closes Redis connection if used
func (f *RepositoryFactory) Close() error {
	if f.redisClient != nil {
		return redisrepo.CloseRedisClient(f.redisClient)
	}
	if f.dbPool != nil {
		f.dbPool.Close()
	}
	return nil
}

// HealthCheck checks Redis connection health
func (f *RepositoryFactory) HealthCheck(ctx context.Context) error {
	if f.useRedis && f.redisClient != nil {
		return f.redisClient.Ping(ctx).Err()
	}
	return nil
}

