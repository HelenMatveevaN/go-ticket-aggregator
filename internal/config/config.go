//Парсер конфига

package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	App 				AppConfig 			`yaml:"app"`
	HTTP 				HTTPConfig 			`yaml:"http"`
	Postgres 			PostgresConfig		`yaml:"postgres"`
	Kafka				KafkaConfig			`yaml:"kafka"`
}

type AppConfig struct {
	Name				string				`yaml:"name" env-default:"ticket-app"`
	Version				string				`yaml:"version" env-default:"1.0.0"`
	Env					string				`yaml:"env" env-default:"development" env:"APP_ENV"`
}

type HTTPConfig struct {
	Port 				string				`yaml:"port" env-default:"8080" env:"HTTP_PORT"`
	Timeout 			time.Duration		`yaml:"timeout" env-default:"4s"`
	ShutdownTimeout		time.Duration		`yaml:"shutdown_timeout" env-default:"5s"`
}

type PostgresConfig struct {
	// Если в системе задана DATABASE_URL, она полностью перекроет строку из YAML
	URL					string				`yaml:"url" env:"DATABASE_URL" env-required:"true"`
	MaxConns			int32				`yaml:"max_conns" env-default:"50"`
	MinConns			int32				`yaml:"min_conns" env-default:"10"`
	MaxConnIdleTime		time.Duration		`yaml:"max_conn_idle_time" env-default:"15m"`
}

type KafkaConfig struct {
	// Тег env-layout:"json" позволяет передавать слайс брокеров через ENV как ["localhost:9092"]
	Brokers				[]string			`yaml:"order_created" env:"KAFKA_TOPIC_ORDER_CREATED" env-default:"orders.order-created"`
	ConsumerGroup		string				`yaml:"ticket_reserved" env:"KAFKA_TOPIC_TICKET_RESERVED" env-default:"tickets.ticket_reserved"`
	PaymentProcessed	string				`yaml:"payment_processed" env:"KAFKA_TOPIC_PAYMENT_PROCESSED" env-default:"payments.payment-processed"`
}