package main

import (
	"log"

	"go-ticket-aggregator/internal/config"
)

func main(){
	//load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	//logging
	log.Printf("[INIT] Starting application: %s (version: %s)", cfg.App.Name, cfg.App.Version)
	log.Printf("[INIT] Running environment: [%s]", cfg.App.Env)
	log.Printf("[INIT] HTTP Server config -> : Port: %s, Timeout: %s", cfg.HTTP.Port, cfg.HTTP.Timeout)
	log.Printf("[INIT] Postgres config -> Pool Max Conns: %d, Min Conns: %d", cfg.Postgres.MaxConns, cfg.Postgres.MinConns)
	log.Printf("[INIT] Kafka config -> Brokers: %v, Group: %s", cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup)
}