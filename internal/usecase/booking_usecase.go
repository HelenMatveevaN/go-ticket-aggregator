//Описываем доменные модели и интерфейсы Кассы

/*
Нам нужны:Доменная структура билета.Интерфейс репозитория 
(который под капотом будет использовать наш txManager и SELECT FOR UPDATE).
*/

package usecase

import (
	"context"
	"fmt"

	"go-ticket-aggregator/internal/domain"
)

type BookingUseCase struct {
	txManager  domain.TransactionManager // Используем интерфейс из домена
	ticketRepo domain.TicketRepository    // Используем интерфейс из домена
}

func NewBookingUseCase(tx domain.TransactionManager, ticketRepo domain.TicketRepository) *BookingUseCase {
	return &BookingUseCase{
		txManager:  tx,
		ticketRepo: ticketRepo,
	}
}

// HoldTicket — конкурентное бронирование билета через доменные правила
func (u *BookingUseCase) HoldTicket(ctx context.Context, eventID int64) (*domain.Ticket, error) {
	if eventID <= 0 {
		return nil, fmt.Errorf("invalid event id: %d", eventID)
	}

	var reservedTicket *domain.Ticket

	// Просим TxManager открыть транзакцию. 
    // передаем внутрь функцию-замыкание
    // u.txManager.WithinTransaction — мы вызываем менеджера
    // А всё, что идет дальше внутри func(txCtx) — это наша инструкция (письмо для него).
	err := u.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		
		// Вот этот кусок кода НЕ ВЫПОЛНЯЕТСЯ СРАЗУ. 
    	// Это просто текст инструкции: "Сначала локни билет, потом обнови статус".
    	ticket, err := u.ticketRepo.GetAvailableTicketWithLock(txCtx, eventID)
		if err != nil {
			return err
		}

		// 2. Применяем доменное бизнес-правило (переводим статус в held и пишем время)
		if err := ticket.Hold(); err != nil {
			return err
		}

		// 3. Сохраняем измененное состояние доменного объекта в базу данных
		// Передаем ту же txCtx во второй метод репозитория
		if err := u.ticketRepo.UpdateStatus(txCtx, ticket); err != nil {
			return fmt.Errorf("failed to save ticket booking: %w", err)
		}

		reservedTicket = ticket
		return nil
	})

	if err != nil {
		// Оборачиваем ошибку для сохранения контекста, но сохраняем доменную ошибку внутри
		return nil, fmt.Errorf("booking transaction aborted: %w", err)
	}

	return reservedTicket, nil
}





