package infrastructure

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/contest-maker-150/backend/internal/domain"
)

// Database wraps the GORM database connection with additional utilities
type Database struct {
	*gorm.DB
	config *DatabaseConfig
	logger *zap.Logger
}

// NewDatabase creates a new database connection with connection pooling
func NewDatabase(config *DatabaseConfig, zapLogger *zap.Logger) (*Database, error) {
	// Create GORM logger adapter
	gormLogger := logger.New(
		&zapLogAdapter{zapLogger},
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(config.DSN()), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true, // Better performance for read operations
		PrepareStmt:            true, // Cache prepared statements
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)

	zapLogger.Info("Database connection established",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.String("database", config.DBName),
		zap.Int("max_open_conns", config.MaxOpenConns),
	)

	return &Database{
		DB:     db,
		config: config,
		logger: zapLogger,
	}, nil
}

// AutoMigrate runs database migrations for all domain entities
func (d *Database) AutoMigrate() error {
	d.logger.Info("Running database migrations...")
	
	err := d.DB.AutoMigrate(
		&domain.User{},
		&domain.Problem{},
		&domain.Contest{},
		&domain.ContestProblem{},
		&domain.Submission{},
	)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	d.logger.Info("Database migrations completed successfully")
	return nil
}

// HealthCheck verifies the database connection is healthy
func (d *Database) HealthCheck(ctx context.Context) error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// WithContext returns a DB with the given context for tracing
func (d *Database) WithContext(ctx context.Context) *gorm.DB {
	return d.DB.WithContext(ctx)
}

// zapLogAdapter adapts zap logger to GORM's logger interface
type zapLogAdapter struct {
	logger *zap.Logger
}

func (z *zapLogAdapter) Printf(format string, args ...interface{}) {
	z.logger.Sugar().Infof(format, args...)
}
