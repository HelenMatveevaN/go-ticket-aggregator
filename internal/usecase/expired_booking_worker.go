package usecase

import (
	"context"
	"log/slog"
	"time"

	"go-ticket-aggregator/internal/domain"
)

//"Дворнику для работы нужен любой репозиторий, у которого есть метод CancelExpiredBookings"
//type BookingRepository interface {
//	CancelExpiredBookings(ctx context.Context, expireTime time.Time) (int64, error)
//}

// ExpiredBookingWorker — это наш Дворник.
// Он хранит в себе инструменты, которые понадобятся ему для работы.
type ExpiredBookingWorker struct {
	ticketRepo   domain.TicketRepository // Инструмент 1: Репозиторий билетов (чтобы давать команды базе данных)
	logger       *slog.Logger     // Инструмент 2: Логгер (чтобы записывать в консоль, что он сделал)
	interval     time.Duration    // Настройка 1: Как часто просыпаться и махать метлой (например, каждые 30 секунд)
	holdDuration time.Duration    // Настройка 2: Сколько времени билет может быть «зависшим» (например, 15 минут)
}

// NewExpiredBookingWorker — это конструктор (фабрика).
// Он принимает готовые инструменты и собирает из них нашего Дворника.
func NewExpiredBookingWorker(
	repo domain.TicketRepository,
	logger *slog.Logger,
	interval time.Duration,
	holdDuration time.Duration,
) *ExpiredBookingWorker {
	return &ExpiredBookingWorker{
		ticketRepo:   repo,
		logger:       logger,
		interval:     interval,
		holdDuration: holdDuration,
	}
}

// Start запускает бесконечный цикл работы Дворника.
func (w *ExpiredBookingWorker) Start(ctx context.Context) {
	w.logger.Info("Фоновый воркер очистки броней успешно запущен")

	// Создаем тикер — это электронный будильник. 
	// Он будет «звенеть» (посылать сигнал) каждые X секунд (из поля w.interval).
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop() // Когда функция завершится, будильник гарантированно выключится

	// Бесконечный цикл — Дворник заступает на постоянное дежурство
	for {
		select {
		case <-ctx.Done():
			// Сигнал 1: Приложение закрывается. Дворник бросает метлу и уходит.
			w.logger.Info("Дворник завершил работу (получен сигнал остановки)")
			return

		case <-ticker.C:
			// Сигнал 2: Будильник зазвенел! Пора убираться.
			w.processExpiredBookings(ctx)
		}
	}
}

// processExpiredBookings — это внутренняя функция-чернорабочий.
// Она вычисляет просроченные билеты и просит базу данных их очистить.
func (w *ExpiredBookingWorker) processExpiredBookings(ctx context.Context) {
	// 1. Считаем точку отсчета: текущее время МИНУС 15 минут (w.holdDuration)
	expireTime := time.Now().Add(-w.holdDuration)

	// 2. Отдаем команду репозиторию. Метод вернет количество очищенных строк.
	rowsAffected, err := w.ticketRepo.CancelExpiredBookings(ctx, expireTime)
	if err != nil {
		// Если база данных вернула ошибку, записываем её в журнал с уровнем Error!
		w.logger.Error("Дворник не смог очистить просроченные брони", 
			slog.String("error", err.Error()),
		)
		return
	}

	// 3. Если Дворник что-то нашел и убрал, пишем об этом радостную новость в лог
	if rowsAffected > 0 {
		w.logger.Info("Дворник успешно освободил зависшие билеты", 
			slog.Int64("count", rowsAffected),
		)
	}
}

