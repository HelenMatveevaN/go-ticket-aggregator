package main

import (
	"context"
	"fmt"
	"log"
	//"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	//"time"

	"github.com/go-redis/redis/v8"

	"go-ticket-aggregator/internal/config"
	"go-ticket-aggregator/internal/repository"
	"go-ticket-aggregator/internal/usecase"
)

func main(){
	//load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	//create context with cancel by signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// =========================================================================
	// DEPENDENCY INJECTION (DI ГРАФ)
	// =========================================================================
	//DI-граф: Конфиг ➔ dbPool ➔ txManager ➔ ticketRepo ➔ ticketUseCase

	//рождение пула (подготовка "линий")
	dbPool, err := repository.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		//log.Fatalf("Failed to initialize Postgres: %v", err)
		log.Printf("[WARNING] Postgres pool cannot connect: %v. Running in offline-mode.", err)
	}

	txManager := repository.NewPostgresTxManager(dbPool) //DI-2: init менеджер транзакций
	ticketRepo := repository.NewPostgresTicketRepository(txManager) //DI-3: create репозитория билетов, push в него txManager

	//orderRepo  := repository.NewPostgresOrderRepository(txManager)
	//outboxRepo := repository.NewPostgresOutboxRepository(txManager)

	log.Printf("[SERVER] Application successfully initialized in [%s] mode.", cfg.App.Env)

	// =========================================================================
	// ИНТЕГРАЦИОННОЕ ТЕСТИРОВАНИЕ МЕГА-ТРАНЗАКЦИИ
	// =========================================================================

	/*if dbPool != nil {
		// Сбросим статус билета перед тестом (для удобства повторных запусков)
		testTicketID := "44444444-1111-1111-1111-111111111111"
		resetQuery := "UPDATE tickets SET status = 'available' WHERE id = $1;"
		_, _ = dbPool.Exec(ctx, resetQuery, testTicketID)

		log.Println("\n--- МЕГА-ТЕСТ: ПОКУПКА БИЛЕТА + ЗАКАЗ + OUTBOX ---")
		
		err = txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
			// Шаг 1: Извлекаем и блокируем билет через SELECT FOR UPDATE
			ticket, err := ticketRepo.GetAvailableTicketWithLock(txCtx, testTicketID)
			if err != nil {
				return err
			}
			log.Printf("[MEGA-TEST] 1. Билет заблокирован! Статус: %s, Цена: %.2f", ticket.Status, ticket.Price.Amount)

			// Переводим доменную модель билета в статус held
			if err := ticket.Hold(); err != nil {
				return err
			}

			// Сохраняем новый статус билета в базу
			if err := ticketRepo.UpdateStatus(txCtx, ticket); err != nil {
				return err
			}
			log.Println("[MEGA-TEST] 2. Статус билета успешно обновлен в базе на 'held'")

			// Шаг 2: Создаем сущность Заказа (Order) в Домене
			newOrder := &domain.Order{
				ID:          "00000000-0000-0000-0000-000000000001",
				UserID:      "99999999-2222-2222-2222-222222222222", // Наш тестовый юзер из сидов
				TicketID:    ticket.ID,
				Status:      domain.StatusOrderCreated,
				TotalAmount: ticket.Price.Amount,
			}

			// Сохраняем Заказ в базу через репозиторий заказов
			if err := orderRepo.Create(txCtx, newOrder); err != nil {
				return err
			}
			log.Println("[MEGA-TEST] 3. Запись о Заказе успешно создана в таблице 'orders'")

			// Шаг 3: Формируем Outbox-событие для брокера Kafka
			newEvent := &domain.OutboxEvent{
				ID:        "11111111-0000-0000-0000-000000000001",
				EventType: "order.created",
				Payload:   []byte(`{"order_id": "00000000-0000-0000-0000-000000000001", "ticket_id": "44444444-1111-1111-1111-111111111111"}`),
				Status:    domain.OutboxPending,
				TraceID:   "test-trace-id-12345",
			}

			// Сохраняем событие в таблицу outbox_events
			if err := outboxRepo.Save(txCtx, newEvent); err != nil {
				return err
			}
			log.Println("[MEGA-TEST] 4. Событие успешно записано в таблицу 'outbox_events'")

			return nil // Ошибок нет — Менеджер автоматически сделает Сommit для всех 3 таблиц!
		})

		if err != nil {
			log.Printf("[MEGA-TEST] КРИТИЧЕСКИЙ ПРОВАЛ: %v", err)
		} else {
			log.Println("[MEGA-TEST] 🔥 ПОЛНЫЙ УСПЕХ: Все три таблицы атомарно изменены! Транзакция зафиксирована.")
		}
	} else {
		// ИСПРАВЛЕНО: Если база выключена, просто пишем об этом в лог и не падаем!
		log.Println("\n[SERVER] База данных недоступна. Интеграционные тесты мега-транзакции пропущены.")
	}

	*/

	// =========================================================================
	// СБОРКА СЛОЕВ ВИТРИНЫ ПО CLEAN ARCHITECTURE (DI-ГРАФ)
	// =========================================================================
	log.Println("[REDIS] Подключение к изолированной быстрой памяти...")
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis-db:6379",
	})

	// Инициализируем репозиторий и UseCase
	eventRepo := repository.NewEventRepository(dbPool)
	eventUseCase := usecase.NewEventUseCase(eventRepo, rdb)

	// Собираем нашу Кассу (UseCase Блока 2)
	bookingUseCase := usecase.NewBookingUseCase(txManager, ticketRepo)
	_ = bookingUseCase // Гасим ошибку неиспользованной переменной


	// =========================================================================
	// ИНИЦИАЛИЗАЦИЯ HTTP СЕРВЕРА
	// =========================================================================
	server := &http.Server{
		Addr: ":" + cfg.HTTP.Port,
	}

	// =========================================================================
	// паттерн CLOSER (Graceful Shutdown)
	// =========================================================================

	go func() {
		<-ctx.Done() //сюда прилетит сигнал Cntrl+C
		log.Printf("[SHUTDOWN] Graceful shutdown initialized... Closing resources.")

		//даем фикс.вр. на закрытие "хвостов"
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		// step 1: останавливаем входящий HTTP-трафик
		log.Printf("[SHUTDOWN] Step 1: Stopping HTTP/gRPC traffic... (No new requests allowed)")
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("[WARNING] HTTP server shutdown error: %v", err)
		}

		// step 2: закрываем базы данных (только после остановки сервера!)
		log.Printf("[SHUTDOWN] Step 2: Closing Storage layers...")
		dbPool.Close()
		log.Printf("[SHUTDOWN] PostgreSQL connection pool closed cleanly.")
	}()

	// =========================================================================
	// РЕГИСТРАЦИЯ МАРШРУТОВ (ЭНДПОИНТОВ)
	// =========================================================================
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Вызываем бизнес-логику нашей Витрины при каждом GET-запросе
		showcaseData, err := eventUseCase.GetShowcaseEvents(r.Context())
		if err != nil {
			log.Printf("[ERROR] Не удалось отобразить Витрину: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"failed to get events"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(showcaseData)) // Отдаем данные витрины в curl / браузер
	})

	// НОВОЕ: Маршрут Кассы (Бронирование билета)
	// Сюда клиент будет слать POST-запрос вида: /bookings?event_id=5
	http.HandleFunc("/bookings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// строго проверяем HTTP-метод. Бронирование должно быть только через POST
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method not allowed, use POST"}`))
			return
		}

		// Достаем event_id из параметров запроса (например, /bookings?event_id=5)
		eventIDStr := r.URL.Query().Get("event_id")
		
		// Senior-Highload допущение: для простоты теста парсим ID, 
		// в будущем мы заменим это на чтение полноценного JSON-тела (DTO)
		var eventID int64
		_, err := fmt.Sscanf(eventIDStr, "%d", &eventID)
		if err != nil || eventID <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid or missing event_id"}`))
			return
		}

		// Вызываем нашу транзакционную Кассу, созданную сегодня!
		ticket, err := bookingUseCase.HoldTicket(r.Context(), eventID)
		if err != nil {
			log.Printf("[ERROR] Ошибка бронирования: %v", err)
			w.WriteHeader(http.StatusConflict) // 409 Conflict — идеальный статус для гонки данных
			w.Write([]byte(`{"error":"ticket already held or sold"}`))
			return
		}

		// Отдаем клиенту успешный ответ с деталями его брони
		w.WriteHeader(http.StatusCreated) // 201 Created
		fmt.Fprintf(w, `{"message":"success","ticket_id":"%s","status":"%s"}`, ticket.ID, ticket.Status)
	})

	// =========================================================================
	// ЗАПУСК СЕРВЕРА
	// =========================================================================
	log.Println("\n[SERVER] Работа завершена. Нажмите Ctrl+C для проверки Closer...")

	// ListenAndServe блокирует поток main и держит приложение запущенным
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}	
}