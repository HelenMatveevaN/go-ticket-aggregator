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
	//1 - load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("[INIT] Starting: %s (version: %s)", cfg.App.Name, cfg.App.Version)
	log.Printf("[INIT] Running environment: [%s]", cfg.App.Env)
	log.Printf("[INIT] HTTP Server config -> : Port: %s, Timeout: %s", cfg.HTTP.Port, cfg.HTTP.Timeout)
	log.Printf("[INIT] Postgres config -> Pool Max Conns: %d, Min Conns: %d", cfg.Postgres.MaxConns, cfg.Postgres.MinConns)
	log.Printf("[INIT] Kafka config -> Brokers: %v, Group: %s", cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup)

	//2 - create context with cancel by signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	//3 - запуск пула соед-ий v4
	dbPool, err := repository.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		log.Fatalf("Failed to initialize Postgres: %v", err)
	}

	//4 - паттерн Closer (Graceful Shutdown)
	go func() {
		<-ctx.Done() //сюда прилетит сигнал Cntrl+C
		log.Printf("[SHUTDOWN] Graceful shutdown initialized... Closing resources.")

		//даем фикс.вр. на закрытие "хвостов"
		_, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		//закрываем пул соед-ий с базой
		dbPool.Close()
		log.Printf("[SHUTDOWN] PostgreSQL connection pool closed cleanly.")
	}()

	log.Printf("[SERVER] Application is running on port %s. Press Cntrl+C to stop.", cfg.HTTP.Port)

	//держим процесс заблокированным, пока контекст не отменится
	<-ctx.Done()

	//небольшая пауза, чтобы горутина успела допечатать логи закрытия пула
	time.Sleep(200 * time.Millisecond)
	log.Println("[SERVER] Application stopped.")
}