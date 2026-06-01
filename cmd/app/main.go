package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-ticket-aggregator/internal/config"
	"go-ticket-aggregator/internal/infrastructure/repository"
)

func main(){
	//load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("[INIT] Starting: %s (version: %s)", cfg.App.Name, cfg.App.Version)
	log.Printf("[INIT] Running environment: [%s]", cfg.App.Env)
	log.Printf("[INIT] HTTP Server config -> : Port: %s, Timeout: %s", cfg.HTTP.Port, cfg.HTTP.Timeout)
	log.Printf("[INIT] Postgres config -> Pool Max Conns: %d, Min Conns: %d", cfg.Postgres.MaxConns, cfg.Postgres.MinConns)
	log.Printf("[INIT] Kafka config -> Brokers: %v, Group: %s", cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup)

	//create context with cancel by signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// =========================================================================
	// DEPENDENCY INJECTION (DI ГРАФ)
	// =========================================================================
	//DI-граф: Конфиг ➔ dbPool ➔ txManager ➔ ticketRepo ➔ ticketUseCase
	dbPool, err := repository.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		log.Fatalf("Failed to initialize Postgres: %v", err)
	}

	txManager := repository.NewPostgresTxManager(dbPool) //DI-2: init менеджер транзакций
	ticketRepo := repository.NewPostgresTicketRepository(txManager) //DI-3: create репозитория билетов, push в него txManager

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
	// ИНТЕГРАЦИОННОЕ ТЕСТИРОВАНИЕ ТРАНЗАКЦИЙ И БЛОКИРОВОК
	// =========================================================================

	testTicketID := "44444444-1111-1111-1111-111111111111"

	log.Println("\n--- ТЕСТ 1: УСПЕШНОЕ ХОЛДИРОВАНИЕ В ТРАНЗАКЦИИ ---")

	//Запускаем транзакцию:
	err = txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		ticket, err := ticketRepo.GetAvailableTicketWithLock(txCtx, testTicketID)
		if err != nil {
			return err
		}
		log.Printf("[TEST 1] Билет успешно заблокирован! Текущий статус в базе: %s, Цена: %.2f %s", 
				ticket.Status, ticket.Price.Amount, ticket.Price.Currency)

		if err := ticket.Hold(); err!= nil {
			return err
		}

		if err := ticketRepo.UpdateStatus(txCtx, ticket); err != nil {
			return err
		}
		log.Printf("[TEST 1] Статус билета изменен на 'held' внутри транзакции. Ожидаем автоматический Commit...")
		return nil
	})

	if err != nil {
		log.Printf("[TEST 1] ПРОВАЛ: %v", err)
	} else {
		log.Println("[TEST 1] УСПЕХ: Транзакция зафиксирована (Commit)!")
	}

	log.Println("\n--- ТЕСТ 2: ИМИТАЦИЯ ОШИБКИ И ОТКАТА (ROLLBACK) ---")
	err = txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		ticket, err := ticketRepo.GetAvailableTicketWithLock(txCtx, testTicketID)
		if err != nil {
			log.Printf("[TEST 2] Ожидаемая ошибка блокировки: %v (билет уже занят)", err)
			return err // Возвращаем ошибку, чтобы запустить Rollback
		}

		_ = ticket
		return nil
	})
	log.Printf("[TEST 2] Результат транзакции: %v (Если тут ошибка — значит Rollback отработал штатно)", err)

	// =========================================================================
	// РАБОТА СЕРВЕРА
	// =========================================================================
	log.Println("\n[SERVER] Тесты завершены. Приложение продолжает работу. Нажмите Ctrl+C для проверки Closer...")
	
	<-ctx.Done() // Блокируем main, пока не нажмем Ctrl+C
	time.Sleep(200 * time.Millisecond)
	log.Println("[SERVER] Application stopped.")
}