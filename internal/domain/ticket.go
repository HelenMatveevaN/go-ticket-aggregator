package domain

import (
	"errors"
	"time"
)

//Домен билета

//доменные ошибки, понятные бизнес-логике
var (
	ErrTicketNotFound = errors.New("ticket not found")
	ErrTicketAlreadyHeld = errors.New("ticket is already held or sold")
)

//value object
type Price struct {
	Amount float64
	Currency string
}

//	это entite (сущность). у нее есть уникальный id и жизненный цикл
type Ticket struct {
	ID        string
	ZoneID    string
	SeatID    *string
	Status    string
	Price     Price // Внедряем наш Value Object
	Version   int
	UpdatedAt time.Time
}

//доменная проверка: можно забронировать билет?
func (t *Ticket) CanBeHeld() bool {
	return t.Status == "available"
}

//переводим сущность в состояние брони
func (t *Ticket) Hold() error {
	if !t.CanBeHeld(){
		return ErrTicketAlreadyHeld
	}
	t.Status = "held"
	t.UpdatedAt = time.Now()
	return nil
}