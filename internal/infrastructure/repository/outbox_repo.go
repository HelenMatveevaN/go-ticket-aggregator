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

// GetPendingEvents вытаскивает пачку (батч) неотправленных событий из базы
// Выгрузка посылок
func (r *PostgresOutboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	// Сортируем по времени создания (ASC), чтобы соблюдать порядок FIFO 
	//(первым пришел — первым ушел)
	query := `
		SELECT id, event_type, payload, status, trace_id, created_at
		FROM outbox_events
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1;
	`

	rows, err := r.txManager.GetQueryable(ctx).Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to select pending outbox events: %w", err)
	}
	defer rows.Close()

	var events []*domain.OutboxEvent

	// Итерируемся по строкам из базы данных
	for rows.Next(){
		var event domain.OutboxEvent
		err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.Payload,
			&event.Status,
			&event.TraceID,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan outbox event row: %w", err)
		}
		events = append(events, &event)
	}

	//Проверяем, не прервался ли обход из-за ошибок субд
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return events, nil
}

// MarkAsProcessed ставит штамп "processed" на событие, которое улетело в Kafka
// Закрытие задачи
// Метод вызывается, когда фоновый воркер успешно отправил сообщение в брокер Кафка,
// и брокер прислал подтверждение
func (r *PostgresOutboxRepository) MarkAsProcessed(ctx context.Context, id string) error {
	query := `
		UPDATE outbox_events
		   SET status = 'processed', processed_at = NOW()
		WHERE id = $1;
	`

	_, err := r.txManager.GetQueryable(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event as processed: %w", err)
	}

	return nil
}

