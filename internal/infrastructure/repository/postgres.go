package repository

import (
	"context"
	"fmt"
	"log"

	"go-ticket-aggregator/internal/config"

	"github.com/jackc/pgx/v4/pgxpool"
)

//NewPostgresPool инициализирует пул соединений v4
func NewPostgresPool(ctx context.Context, cfg config.PostgresConfig) (*pgxpool.Pool, error) {
	//1 - парсим базовую строку подключения
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database URL: %w", err)
	}

	//2 - тюнинг под Highload (в v4 - тип int)
	poolConfig.MaxConns = int32(cfg.MaxConns)
	poolConfig.MinConns = int32(cfg.MinConns)
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	//3 - создаем пул
	pool, err := pgxpool.ConnectConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to postgres: %w", err)
	}

	//4 - ping (обязательно), проверка доступности базы
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	log.Println("[POSTGRES] ⚡ Connection pool successfully initialized and pinged!")
	return pool, nil
}