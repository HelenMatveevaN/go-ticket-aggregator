//здесь будет жить чистый SQL-запрос к Postgres
package repository

import (
	"context"
	"fmt"
	"go-ticket-aggregator/internal/domain"

	"github.com/jackc/pgx/v4/pgxpool"
)

// EventRepository управляет данными мероприятий в Postgres
type EventRepository struct {
	pool *pgxpool.Pool
}

// NewEventRepository создает новый экземпляр репозитория
func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

// GetActiveEvents достает все активные мероприятия из базы данных
func (r *EventRepository) GetActiveEvents(ctx context.Context) ([]domain.Event, error) {
	query := `SELECT id, title, description, status FROM events`
	
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Status)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}