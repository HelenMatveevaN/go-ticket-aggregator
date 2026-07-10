//Репозиторий билетов
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-ticket-aggregator/internal/domain"

	"github.com/jackc/pgx/v4"	
)

type PostgresTicketRepository struct {
	txManager domain.TransactionManager
}

//конструктор репозитория билетов, функция-фабрика
func NewPostgresTicketRepository(txManager domain.TransactionManager) *PostgresTicketRepository {
	return &PostgresTicketRepository{txManager: txManager}
}

//ищет свободный билет по его ID и блокирует строку в БД
func (r *PostgresTicketRepository) GetAvailableTicketWithLock(ctx context.Context, eventID int64) (*domain.Ticket, error) {

	query := `
		SELECT t.id, t.zone_id, t.seat_id, t.status, t.version, z.price
		FROM tickets t
		JOIN zones z 
		  ON t.zone_id = z.id
		WHERE t.id = $1 AND t.status = 'available'
		FOR UPDATE;
	`

	// Проверка Менеджера Транзакций (Type Assertion)
	tm, ok := r.txManager.(*PostgresTxManager)
	if !ok {
		return nil, fmt.Errorf("unexpected transaction manager type")
	}

	var ticket domain.Ticket
	var priceCents int64

	// ТАК вызывается тумблер! Мы передаем ему наш ctx (который на самом деле txCtx)
	queryableObject := tm.GetQueryable(ctx)

	// Безопасно достаем транзакцию или пул через хелпер GetQueryable
	// Выполняем SQL-запрос через тот объект, который вернул тумблер
	err := queryableObject.QueryRow(ctx, query, eventID).Scan(
		&ticket.ID,
		&ticket.ZoneID,
		&ticket.SeatID,
		&ticket.Status,
		&ticket.Version,
		&priceCents,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { //Если билет не найден (или уже занят)
			return nil, domain.ErrTicketAlreadyHeld
		}
		return nil, fmt.Errorf("failed to select ticket with lock: %w", err)
	}

	ticket.Price = domain.Price{
		AmountCents: priceCents, // Меняем на AmountCents
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

// CancelExpiredBookings находит и массово освобождает просроченные брони билетов.
// Этот метод будет вызываться Дворником фоном, без открытия больших транзакций.
func (r *PostgresTicketRepository) CancelExpiredBookings(ctx context.Context, expireTime time.Time) (int64, error) {
	// Высокопроизводительный запрос: переводим статус held -> available, сбрасываем время блокировки,
	// инкрементируем версию (на случай, если кто-то параллельно пытался его купить)
	// и возвращаем количество измененных строк.
	query := `
		UPDATE tickets
		SET status = 'available', 
		    locked_at = NULL, 
		    version = version + 1, 
		    updated_at = NOW()
		WHERE status = 'held' AND locked_at < $1;
	`

	tm, ok := r.txManager.(*PostgresTxManager)
	if !ok {
		return 0, fmt.Errorf("unexpected transaction manager type")
	}

	// Вызываем Exec через наш GetQueryable. Так как Дворник работает вне транзакции,
	// тумблер GetQueryable автоматически вернет обычный pool соединений.
	cmdTag, err := tm.GetQueryable(ctx).Exec(ctx, query, expireTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cancel expired bookings: %w", err)
	}

	// pgx возвращает специальный тег, из которого можно узнать количество затронутых строк
	return cmdTag.RowsAffected(), nil
}