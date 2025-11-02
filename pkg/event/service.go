package event

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// EventService содержит всю бизнес-логику работы с событиями
// Он решает, можно ли создать новое событие, можно ли завершить существующее и т.д.
type EventService struct {
	// repo — это репозиторий, который работает с базой данных
	// Сервис использует его для всех операций с данными
	repo *EventRepository
}

// NewEventService создаёт новый сервис для работы с событиями
// Нужно просто передать ему репозиторий, который уже знает, как работать с базой
func NewEventService(repo *EventRepository) *EventService {
	return &EventService{repo: repo}
}

// Start запускает новое событие указанного типа
// Но делает это умно: сначала проверяет, нет ли уже активного события такого типа
// Если есть — ничего не делает, просто возвращает существующее событие
// Если нет — создаёт новое
func (s *EventService) Start(ctx context.Context, eventType string) (*Event, error) {
	// Сначала проверяем, нет ли уже активного события этого типа
	active, err := s.repo.FindActive(ctx, eventType)
	if err != nil {
		// Если произошла ошибка при поиске — возвращаем её
		return nil, err
	}

	// Если активное событие уже есть — просто возвращаем его
	// Не создаём дубликат, как и требуется в ТЗ
	if active != nil {
		return active, nil
	}

	// Если активного события нет — создаём новое
	return s.repo.Create(ctx, eventType)
}

// Finish завершает активное событие указанного типа
// Если такого события нет — вернёт ошибку
// Если есть — завершит его и вернёт обновлённое событие
func (s *EventService) Finish(ctx context.Context, eventType string) (*Event, error) {
	// Просто просим репозиторий завершить событие
	// Репозиторий сам вернёт ошибку, если события не найдётся
	event, err := s.repo.Finish(ctx, eventType)
	if err == mongo.ErrNoDocuments {
		// Если события нет — возвращаем ошибку, которую потом обработает handler
		return nil, mongo.ErrNoDocuments
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}

// List возвращает список событий с учетом фильтров
// Параметры:
//   - offset: смещение от начала списка (0 = с самого начала)
//   - limit: максимальное количество событий (0 = без ограничения, максимум 100)
//   - eventType: фильтр по типу события (пустая строка = без фильтра)
//
// События отсортированы по времени начала в порядке убывания (descending)
func (s *EventService) List(ctx context.Context, offset int, limit int, eventType string) ([]Event, error) {
	// Просим репозиторий вернуть события с учетом фильтров
	return s.repo.List(ctx, offset, limit, eventType)
}
