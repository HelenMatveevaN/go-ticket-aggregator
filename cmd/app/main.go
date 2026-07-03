package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	//"github.com/jackc/pgx/v4"
	"github.com/go-redis/redis/v8"

	//"go-ticket-aggregator/internal/domain"
	"go-ticket-aggregator/internal/config"
	"go-ticket-aggregator/internal/repository"
	"go-ticket-aggregator/internal/usecase"
)

//мероприятие
/*type Event struct {
	ID             string
	Title          string
	Category       string
	StartTime      time.Time
	Location       string
	MinPrice       float64
	Status         string
}*/

func main(){
	//load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	/*log.Printf("[INIT] Starting: %s (version: %s)", cfg.App.Name, cfg.App.Version)
	log.Printf("[INIT] Running environment: [%s]", cfg.App.Env)
	log.Printf("[INIT] HTTP Server config -> : Port: %s, Timeout: %s", cfg.HTTP.Port, cfg.HTTP.Timeout)
	log.Printf("[INIT] Postgres config -> Pool Max Conns: %d, Min Conns: %d", cfg.Postgres.MaxConns, cfg.Postgres.MinConns)
	log.Printf("[INIT] Kafka config -> Brokers: %v, Group: %s", cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup)
	*/

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

	//txManager := repository.NewPostgresTxManager(dbPool) //DI-2: init менеджер транзакций
	//ticketRepo := repository.NewPostgresTicketRepository(txManager) //DI-3: create репозитория билетов, push в него txManager

	//orderRepo  := repository.NewPostgresOrderRepository(txManager)
	//outboxRepo := repository.NewPostgresOutboxRepository(txManager)

	// =========================================================================
	// паттерн CLOSER (ГАРАНТИРОВАННЫЙ ПОРЯДОК ЗАВЕРШЕНИЯ) - (Graceful Shutdown)
	// =========================================================================

	go func() {
		<-ctx.Done() //сюда прилетит сигнал Cntrl+C
		log.Printf("[SHUTDOWN] Graceful shutdown initialized... Closing resources.")

		//даем фикс.вр. на закрытие "хвостов"
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		//step1 : останавливаем вх.потоки (серверы, консьюмеры)
		//далее вставим сюда http.Shutdown
		log.Printf("[SHUTDOWN] Step 1: Stopping HTTP/gRPC traffic... (No new requests allowed)")
		time.Sleep(500 * time.Millisecond)

		//step2 : закрываем слои хранения и базы данных
		//но - только после того, как вх.поток иссяк!
		log.Printf("[SHUTDOWN] Step 2: Closing Storage layers...")
		dbPool.Close()
		log.Printf("[SHUTDOWN] PostgreSQL connection pool closed cleanly.")

		_ = shutdownCtx //заглушка для будущих операций с переменной
	}()

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
		Addr: "localhost:6379",
	})

	// Инициализируем репозиторий (Перенесли тяжелый SQL-запрос в internal/repository)
	eventRepo := repository.NewEventRepository(dbPool)

	// Инициализируем UseCase (Перенесли логику "бронежилета" в internal/usecase)
	eventUseCase := usecase.NewEventUseCase(eventRepo, rdb)

	// Вызываем бизнес-логику нашей Витрины
	fmt.Println("\n=============================================")
	fmt.Println("         --- НАША ВИТРИНА МЕРОПРИЯТИЙ ---    ")
	fmt.Println("=============================================")

	showcaseData, err := eventUseCase.GetShowcaseEvents(ctx)
	if err != nil {
		log.Printf("[ERROR] Не удалось отобразить Витрину: %v", err)
	} else {
		fmt.Print(showcaseData) // Просто выводим результат работы UseCase
	}

	// =========================================================================
	// РАБОТА СЕРВЕРА
	// =========================================================================
	log.Println("\n[SERVER] Работа завершена. Нажмите Ctrl+C для проверки Closer...")
	<-ctx.Done()
}