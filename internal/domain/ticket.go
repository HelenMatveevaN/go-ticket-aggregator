//Домен билета

package domain

import (
	"context"
	"errors"
	"time"
)

// Статусы билета
const (
	StatusAvailable = "available"
	StatusHeld      = "held"
	StatusSold      = "sold"
)

//доменные ошибки, понятные бизнес-логике
var (
	ErrTicketAlreadyHeld = errors.New("билет уже забронирован или продан")
	ErrTicketNotFound = errors.New("билет не найден")
)

type Price struct {
	AmountCents int64  // храним деньги в копейках (например, 150000 вместо 1500.00)
	Currency    string
}

// Ticket описывает конкретное место на мероприятии
type Ticket struct {
	ID        string
	ZoneID    string
	SeatID    string
	EventID   int64     // Привязка к мероприятию
	Status    string    // "available", "held", "sold"
	Version   int
	Price     Price
	LockedAt  time.Time 
	UpdatedAt time.Time // Добавили поле, чтобы метод Hold() работал
}

// Hold переводит билет в статус брони на уровне бизнес-логики
func (t *Ticket) Hold() error {
	if t.Status != StatusAvailable {
		return ErrTicketAlreadyHeld
	}
	now := time.Now()
	t.Status = StatusHeld
	t.LockedAt = now
	t.UpdatedAt = now
	return nil
}

// TicketRepository — интерфейс репозитория, который лежит в домене.
// Слой инфраструктуры (repository) будет обязан его реализовать.
type TicketRepository interface {
	GetAvailableTicketWithLock(ctx context.Context, eventID int64) (*Ticket, error)
	UpdateStatus(ctx context.Context, ticket *Ticket) error

	// НОВЫЙ МЕТОД ДЛЯ ДВОРНИКА:
	// Находит все билеты со статусом 'held', у которых locked_at меньше, чем expireTime,
	// и переводит их обратно в 'available'. Возвращает количество очищенных билетов.
	CancelExpiredBookings(ctx context.Context, expireTime time.Time) (int64, error)
}

// TransactionManager — интерфейс управления транзакциями, который видит домен
//type TransactionManager interface {
//	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
//}