package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	_ "github.com/EgorKo25/EasyNote/migrations"
	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/logger"
	"github.com/EgorKo25/EasyNote/src/planner"
)

func NewDatabase(ctx context.Context, cfg config.Database) (*Database, error) {
	connectionString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("parse postgres pool config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping postgres pool: %w", err)
	}

	return &Database{pool: pool, cfg: cfg}, nil
}

type Database struct {
	pool *pgxpool.Pool
	cfg  config.Database
}

func (d *Database) Migrate(ctx context.Context) error {
	if err := goose.SetDialect(d.cfg.MigrationDialect); err != nil {
		return fmt.Errorf("set goose dialect %q: %w", d.cfg.MigrationDialect, err)
	}

	sqlDB := stdlib.OpenDBFromPool(d.pool)

	err := goose.UpContext(ctx, sqlDB, ".")
	if closeErr := sqlDB.Close(); closeErr != nil {
		logger.Get("database/postgresql").Error("cannot close migration sql database", closeErr)
	}

	if err != nil {
		return fmt.Errorf("run database migrations: %w", err)
	}

	return nil
}

func (d *Database) WithinTransaction(
	ctx context.Context,
	action func(ctx context.Context, storage planner.TransactionStorage) error,
) error {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "planner")

	if err = action(ctx, newPlannerTransactionStorage(transaction)); err != nil {
		return err
	}

	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func rollbackTransaction(ctx context.Context, transaction pgx.Tx, operation string) {
	if rollbackErr := transaction.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
		logger.Get("database/postgresql").Error("cannot rollback "+operation+" transaction", rollbackErr)
	}
}
