package domain

import (
	"context"
	"time"
)

// Кастомный тип для контроля жизненного цикла события в базе
type OutboxStatus string

const (
	OutboxPending   OutboxStatus = "pending"   // Ожидает отправки воркером в Кафку
	OutboxProcessed OutboxStatus = "processed" // Успешно улетело в брокер
	OutboxFailed    OutboxStatus = "failed"    // Упало с ошибкой
)

// OutboxEvent — доменная модель события (наша Entity)
type OutboxEvent struct {
	ID        string
	EventType string
	Payload   []byte // Сами данные события (например, ID заказа), переведенные в текст JSON
	Status    OutboxStatus
	TraceID   string // Задел на будущее для распределенного трейсинга в Grafana/Jaeger
	CreatedAt time.Time
}

// OutboxRepository — контракт.
// Домен требует от инфраструктуры уметь сохранять такие события в таблицу.
type OutboxRepository interface {
	Save(ctx context.Context, event *OutboxEvent) error
}