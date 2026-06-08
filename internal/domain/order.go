package domain

import (
	"context"
	"errors"
	"time"
)

// Ошибки, которые могут произойти с заказом по правилам бизнеса
var ErrOrderAlreadyPaid = errors.New("order is already paid or cancelled")

// Кастомный тип для статусов заказа, чтобы никто не передал туда случайную строку
type OrderStatus string

const (
	StatusOrderCreated   OrderStatus = "created"
	StatusOrderPaid      OrderStatus = "paid"
	StatusOrderCancelled OrderStatus = "cancelled"
)

// Order — доменная модель заказа (наша Entity)
type Order struct {
	ID          string
	UserID      string
	TicketID    string
	Status      OrderStatus
	TotalAmount float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OrderRepository — контракт (интерфейс). 
// Домен говорит: «Мне плевать, какая будет БД, но она обязана уметь сохранять Заказ».
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
}