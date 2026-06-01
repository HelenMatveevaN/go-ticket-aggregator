package repository

//Создаем Менеджер Транзакций tx_manager.go

//сердце инфраструктурного слоя

//изолирует технические детали базы данных от бизнес-логики

//позволяет запускать транзакции в postgres и незаметно прокидывать их через контекст
//чтобы бизнес-логика не зависела от sql-библиотек

//позволяет выполнять sql-запросы как внутри транзакции, так и без нее

import (
	"context"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"	
)

// секретный ключ контекста
type txKey struct {}

//интерфейс (полиморфизм) - работает и с пулом, и с транзакцией
type Queryable interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type PostgresTxManager struct {
	pool *pgxpool.Pool
}

//Конструктор менеджера транзакций
func NewPostgresTxManager(pool *pgxpool.Pool) *PostgresTxManager {
	return &PostgresTxManager{pool: pool}
}

//запускает транзакцию Postgres и передает её в контекст
//реализует паттерн автоматического управления транзакциями через контекст
func (tm *PostgresTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error{
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx) //если транз-я уже идет, просто выполняем ф-ю дальше
	}

	//запускаем атомарную транзацию (если еще не была запущена)
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to start transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)
	err = fn(txCtx)

	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("transaction error: %v, rollback failed: %w", err, rbErr)
		}
		return err
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", cmErr)
	}

	return nil
}

//Магия переключения (между пулом и транзакцией - в зависимости от того, что нашли)
func (tm *PostgresTxManager) GetQueryable(ctx context.Context) Queryable {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx //возвращаем транзакцию
	}
	return tm.pool //возвращаем пул
}



