//репозиторий заказов

package repository

import (
	"context"
	"fmt"

	"go-ticket-aggregator/internal/domain"
)


type PostgresOrderRepository struct {
	txManager *PostgresTxManager
}

// NewPostgresOrderRepository — конструктор репозитория заказов
func NewPostgresOrderRepository(txManager *PostgresTxManager) *PostgresOrderRepository {
	return &PostgresOrderRepository{txManager: txManager}
}

// Create записывает данные о новом заказе в таблицу orders
func (r *PostgresOrderRepository) Create(ctx context.Context, order *domain.Order) error {
	query := `
		INSERT INTO orders (id, user_id, status, total_amount, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW());
	`
	// Вызываем наш хелпер GetQueryable. 
	// Благодаря ему, если в контексте идет транзакция — заказ запишется внутрь нее.
	_, err := r.txManager.GetQueryable(ctx).Exec(
		ctx, 
		query, 
		order.ID, 
		order.UserID, 
		order.Status, 
		order.TotalAmount,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}
	
	return nil
}