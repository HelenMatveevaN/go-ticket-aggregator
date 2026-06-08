package repository

import (
	"context"
	"fmt"

	"go-ticket-aggregator/internal/domain"
)

type PostgresOutboxRepository struct {
	txManager *PostgresTxManager
}

// NewPostgresOutboxRepository — конструктор репозитория outbox
func NewPostgresOutboxRepository(txManager *PostgresTxManager) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{txManager: txManager}
}

// Save записывает событие в таблицу outbox_events для последующей отправки в Kafka
func (r *PostgresOutboxRepository) Save(ctx context.Context, event *domain.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (id, event_type, payload, status, trace_id, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW());
	`

	// Хелпер GetQueryable автоматически подхватит транзакцию, если UseCase её открыл
	_, err := r.txManager.GetQueryable(ctx).Exec(
		ctx,
		query,
		event.ID,
		event.EventType,
		event.Payload,
		event.Status,
		event.TraceID,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}