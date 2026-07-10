package usecase

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"go-ticket-aggregator/internal/domain"
)

// 1. Создаем фальшивый репозиторий-заглушку (Mock)
type MockBookingRepository struct {
	CalledWithTime time.Time // Переменная, чтобы запомнить, какое время посчитал Дворник
	ReturnRows     int64     // Переменная, чтобы задать, сколько строк якобы очистила база
}

// МЕТОД 1 (Нужен Дворнику): Реальная логика для теста
func (m *MockBookingRepository) CancelExpiredBookings(ctx context.Context, expireTime time.Time) (int64, error) {
	m.CalledWithTime = expireTime // Запоминаем время, с которым пришел Дворник
	return m.ReturnRows, nil      // Возвращаем то количество строк, которое мы настроили в тесте
}

// МЕТОД 2 (Нужен Кассе): Пустая заглушка для компилятора
func (m *MockBookingRepository) GetAvailableTicketWithLock(ctx context.Context, eventID int64) (*domain.Ticket, error) {
	return nil, nil // Метод просто возвращает пустоту, так как Дворник его не использует
}

// МЕТОД 3 (Нужен Кассе): Пустая заглушка для компилятора
func (r *MockBookingRepository) UpdateStatus(ctx context.Context, ticket *domain.Ticket) error {
	return nil
}

// Наш тест
func TestExpiredBookingWorker_Process(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	// Настраиваем нашу заглушку: притворяемся, что база успешно очистила 5 билетов
	mockRepo := &MockBookingRepository{
		ReturnRows: 5,
	}

	// Задаем настройки времени: бронь живет 15 минут
	holdDuration := 15 * time.Minute

	// Создаем Дворника и подсовываем ему наш фальшивый mockRepo вместо реального Postgres
	worker := NewExpiredBookingWorker(mockRepo, logger, 1*time.Second, holdDuration)

	// Запускаем ОДНУ итерацию очистки (тот самый внутренний метод-чернорабочий)
	ctx := context.Background()
	worker.processExpiredBookings(ctx)

	// Проверки (Assertions)
	// Проверяем, что Дворник вообще дошел до репозитория и вызвал его
	if mockRepo.CalledWithTime.IsZero() {
		t.Fatalf("Ожидалось, что Дворник вызовет метод CancelExpiredBookings, но вызова не произошло")
	}

	// Проверяем математику времени: Дворник должен был отнять 15 минут от текущего времени
	expectedTime := time.Now().Add(-holdDuration)
	
	// Считаем разницу между тем, что посчитал Дворник, и тем, что ожидает тест
	diff := expectedTime.Sub(mockRepo.CalledWithTime)

	// Если разница больше 1 секунды (пока тест компилировался и запускался), значит, математика сломана
	if diff > time.Second || diff < -time.Second {
		t.Errorf("Дворник неправильно посчитал время отсечки! Получено: %v, Ожидалось примерно: %v", mockRepo.CalledWithTime, expectedTime)
	}
}