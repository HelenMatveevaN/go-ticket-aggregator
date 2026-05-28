//Парсер конфига

package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
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

	Brokers       		[]string    		`yaml:"brokers" env:"KAFKA_BROKERS" env-separator:","`
	ConsumerGroup 		string      		`yaml:"consumer_group" env:"KAFKA_CONSUMER_GROUP" env-default:"ticket-aggregator-group"`
	Topics        		TopicConfig 		`yaml:"topics"`
}

type TopicConfig struct {
	OrderCreated		string				`yaml:"order_created" env-default:"orders.order-created"`
	TicketReserved		string				`yaml:"ticket_reserved" env-default:"tickets.ticket_reserved"`
	PaymentProcessed	string				`yaml:"payment_processed" env-default:"payments.payment-processed"`
}

//Load инициализирует конфигурацию, собирая данные из .env, YAML и системного окружения
func Load() (*Config, error) {
	var cfg Config //выделение памяти

	// 1. Читаем .env напрямую
	if _, err := os.Stat(".env"); err == nil {
		if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
			return nil, fmt.Errorf("failed to read .env file: %w", err)
		}
	}

	configPath := "config/config.yaml"	

	// 2. YAML exists?
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found at path: %s", configPath)
	}
	
	// 3. Читаем YAML. Метод ReadConfig НЕ затирает поля, которые УЖЕ заполнены,
	// Он сначала прочитает YAML, а затем сам заменит поля на значения из ENV, если они заданы!
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("failed to read or parse config: %w", err)
	}

	return &cfg, nil
}
