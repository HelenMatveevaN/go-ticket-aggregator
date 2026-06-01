package repository

//Репозиторий билетов

import (
	"context"
	"errors"
	"fmt"

	"go-ticket-aggregator/internal/domain"

	"github.com/jackc/pgx/v4"	
)

type PostgresTicketRepository struct {
	txManager *PostgresTxManager //зависимость от менеджера транзакций
}

//конструктор репозитория билетов
func NewPostgresTicketRepository(txManager *PostgresTxManager) *PostgresTicketRepository {
	return &PostgresTicketRepository{txManager: txManager}
}

//выбирает билет и блокирует строку базы через FOR UPDATE
func (r *PostgresTicketRepository) GetAvailableTicketWithLock(ctx context.Context, id string) (*domain.Ticket, error) {

	query := `
		SELECT t.id, t.zone_id, t.seat_id, t.status, t.version, z.price
		FROM ticket t
		JOIN zones z 
		  ON t.zone_id = z.id
		WHERE t.id = $1 AND t.status = 'available'
		FOR UPDATE;
	`

	var ticket domain.Ticket
	var priceAmount float64

	// Магия GetQueryable: метод автоматически выберет транзакцию из контекста, если она там есть!
	err := r.txManager.GetQueryable(ctx).QueryRow(ctx, query, id).Scan(
		&ticket.ID,
		&ticket.ZoneID,
		&ticket.SeatID,
		&ticket.Status,
		&ticket.Version,
		&priceAmount,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTicketAlreadyHeld
		}
		return nil, fmt.Errorf("failed to select ticket with lock: %w", err)
	}

	ticket.Price = domain.Price{
		Amount: priceAmount,
		Currency: "RUB",
	}

	return &ticket, nil
}

//обновляет статус билета в базе данных
func (r *PostgresTicketRepository) UpdateStatus(ctx context.Context, ticket *domain.Ticket) error {
	query := `
		UPDATE tickets 
		SET status = $1, version = version + 1, updated_at = NOW() 
		WHERE id = $2;
	`

	_, err := r.txManager.GetQueryable(ctx).Exec(ctx, query, ticket.Status, ticket.ID)
	if err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}
	return nil
}