//опишем простую структуру самого Мероприятия (модель данных).

package domain

import "time"

// Event описывает сущность Мероприятия в нашей системе
type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartAt     time.Time `json:"start_at"`
	Status      string    `json:"status"`
}
