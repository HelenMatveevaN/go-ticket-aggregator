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
	//Репозиторий зависит не от конкретной базы данных, а от интерфейса управления транзакциями 
	//domain.TransactionManager
	txManager domain.TransactionManager
}

//конструктор репозитория билетов, принимает интерфейс из домена
func NewPostgresTicketRepository(txManager domain.TransactionManager) *PostgresTicketRepository {
	return &PostgresTicketRepository{txManager: txManager}
}

//ищет свободный билет по его ID и блокирует строку в БД
func (r *PostgresTicketRepository) GetAvailableTicketWithLock(ctx context.Context, id string) (*domain.Ticket, error) {

	query := `
		SELECT t.id, t.zone_id, t.seat_id, t.status, t.version, z.price
		FROM tickets t
		JOIN zones z 
		  ON t.zone_id = z.id
		WHERE t.id = $1 AND t.status = 'available'
		FOR UPDATE;
	`

	// Приведение типов (Type Assertion): проверяем, что за интерфейсом скрывается именно PostgresTxManager
	tm, ok := r.txManager.(*PostgresTxManager)
	if !ok {
		return nil, fmt.Errorf("unexpected transaction manager type")
	}

	var ticket domain.Ticket
	var priceAmount float64

	// Безопасно достаем транзакцию или пул через хелпер GetQueryable
	err := tm.GetQueryable(ctx).QueryRow(ctx, query, id).Scan(
		&ticket.ID,
		&ticket.ZoneID,
		&ticket.SeatID,
		&ticket.Status,
		&ticket.Version,
		&priceAmount,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { //Если билет не найден (или уже занят)
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
	//инкремент версии для оптимист.блок-ки
	query := `
		UPDATE tickets 
		SET status = $1, version = version + 1, updated_at = NOW() 
		WHERE id = $2;
	`

	tm, ok := r.txManager.(*PostgresTxManager)
	if !ok {
		return fmt.Errorf("unexpected transaction manager type")
	}

	_, err := tm.GetQueryable(ctx).Exec(ctx, query, ticket.Status, ticket.ID) //просто выполняет команду
	if err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}

	return nil
}