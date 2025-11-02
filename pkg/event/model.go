package event

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// State представляет состояние события — активное оно или уже завершено
type State int

const (
	// Active означает, что событие сейчас активно и ещё не закончилось
	Active State = 0
	// Finished означает, что событие уже завершено
	Finished State = 1
)

// String возвращает строковое представление состояния
func (s State) String() string {
	switch s {
	case Active:
		return "started"
	case Finished:
		return "finished"
	default:
		return "unknown"
	}
}

// Event — это основная модель нашего события
// Хранит всю информацию о том, когда оно началось, когда закончилось, и какой у него тип
type Event struct {
	// ID — уникальный идентификатор события в базе данных
	ID primitive.ObjectID `bson:"_id,omitempty" json:"-"`

	// Type — тип события (например, "login", "logout", "payment" и т.д.)
	// Это позволяет различать разные виды событий
	Type string `bson:"type" json:"type"`

	// State показывает, активно событие или уже завершено
	State State `bson:"state" json:"-"`

	// StartedAt — время, когда событие началось
	// Всегда заполнено, потому что каждое событие должно иметь начало
	StartedAt time.Time `bson:"started_at" json:"-"`

	// FinishedAt — время, когда событие завершилось
	// Может быть null, потому что активные события ещё не имеют времени завершения
	FinishedAt *time.Time `bson:"finished_at,omitempty" json:"-"`
}

// EventResponse представляет событие в формате API согласно OpenAPI контракту
type EventResponse struct {
	ID         string     `json:"id"` // ObjectID как строка
	Type       string     `json:"type"`
	State      string     `json:"state"`                // "started" или "finished"
	StartedAt  time.Time  `json:"startedAt"`            // camelCase
	FinishedAt *time.Time `json:"finishedAt,omitempty"` // camelCase
}

// ToResponse преобразует Event в EventResponse для API ответа
func (e *Event) ToResponse() EventResponse {
	resp := EventResponse{
		ID:        e.ID.Hex(), // Преобразуем ObjectID в строку
		Type:      e.Type,
		State:     e.State.String(),
		StartedAt: e.StartedAt,
	}
	if e.FinishedAt != nil {
		resp.FinishedAt = e.FinishedAt
	}
	return resp
}

// MarshalJSON кастомная сериализация Event в формат EventResponse
func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.ToResponse())
}
