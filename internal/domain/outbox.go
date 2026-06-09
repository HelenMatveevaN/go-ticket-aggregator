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

// OutboxRepository — контракт
// Домен требует от инфраструктуры уметь сохранять такие события в таблицу
// Дополняем контракт для фонового воркера
type OutboxRepository interface {
	Save(ctx context.Context, event *OutboxEvent) error

	//Глаза и руки нашего будущего фонового воркера Kafka
	//Они реализуют гарантию доставки сообщений At-Least-Once (минимум один раз)

	//GetPendingEvents - Сборщик посылок, заставляет приложение раз в секунду заглядывать в таблицу
	//и забирать пачку (батч) из свежих посылок
	GetPendingEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)

	//MarkAsProcessed - Штамп об отправке, Когда воркер забрал событие из базы, 
	//он берет его и отправляет в брокер Кафка.
	//Как только Кафка ответила "order.created", воркер вызывает метод MarkAsProcessed.
	//Меняет статус с pending на processed (обработано)
	MarkAsProcessed(ctx context.Context, id string) error
}