//создадим «мозг» нашей Витрины. 
//Этот слой будет управлять кэшем Redis и репозиторием Postgres.

package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	//"go-ticket-aggregator/internal/domain"
	"go-ticket-aggregator/internal/repository"

	"github.com/go-redis/redis/v8"
)

// EventUseCase управляет бизнес-логикой Витрины
type EventUseCase struct {
	repo *repository.EventRepository
	rdb  *redis.Client
}

// NewEventUseCase собирает наш UseCase вместе
func NewEventUseCase(repo *repository.EventRepository, rdb *redis.Client) *EventUseCase {
	return &EventUseCase{
		repo: repo,
		rdb:  rdb,
	}
}

// GetShowcaseEvents возвращает данные для Витрины с защитой через Redis
func (uc *EventUseCase) GetShowcaseEvents(ctx context.Context) (string, error) {
	cacheKey := "widget:events:list"

	// 1. Ищем снимок на "стикре" в Redis (Cache Hit)
	cachedData, err := uc.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		log.Println("[CACHE HIT] 🚀 Мгновенно отдаем данные из Redis! Слой UseCase защитил БД.")
		return cachedData, nil
	}

	// 2. Если в Redis пусто, будим Postgres (Cache Miss)
	log.Println("[CACHE MISS] 🔍 В Redis пусто. UseCase вызывает репозиторий Postgres...")
	events, err := uc.repo.GetActiveEvents(ctx)
	if err != nil {
		return "", fmt.Errorf("usecase не смог получить события: %w", err)
	}

	// 3. Если база пустая, так и говорим
	if len(events) == 0 {
		return "⚠️ На Витрине пока нет доступных мероприятий.", nil
	}

	// 4. Формируем красивый текстовый снимок
	var buffer string
	for _, e := range events {
		card := fmt.Sprintf("🎭 %s\n📝 Описание: %s\n🟢 Статус: %s\n---------------------------------------------\n",
			e.Title, e.Description, e.Status)
		buffer += card
	}

	// 5. Записываем снимок в Redis на 5 минут для следующих пользователей
	err = uc.rdb.Set(ctx, cacheKey, buffer, 5*time.Minute).Err()
	if err != nil {
		log.Printf("[WARNING] UseCase не смог обновить кэш в Redis: %v", err)
	} else {
		log.Println("[REDIS] 📝 Снимок Витрины успешно обновлен UseCase-ом на 5 минут.")
	}

	return buffer, nil
}