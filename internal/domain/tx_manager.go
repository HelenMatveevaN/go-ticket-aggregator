package domain

import (
	"context"
)

//абстрактный контракт для управления транзакциями
//слой бизнес-логики (UseCase) будет вызывать именно его.

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}