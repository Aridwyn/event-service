package event

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EventRepository отвечает за всю работу с базой данных
// Он знает, как сохранять события, как их находить и обновлять
type EventRepository struct {
	// collection — это ссылка на коллекцию MongoDB, с которой мы работаем
	collection *mongo.Collection
}

// NewEventRepository создаёт новый репозиторий для работы с событиями
// Нужно просто передать ему коллекцию из MongoDB, и он готов к работе
func NewEventRepository(col *mongo.Collection) *EventRepository {
	return &EventRepository{collection: col}
}

// FindActive ищет активное событие указанного типа
// Если такого события нет, вернёт nil без ошибки
// Используется для проверки, не запущено ли уже событие этого типа
func (r *EventRepository) FindActive(ctx context.Context, eventType string) (*Event, error) {
	var event Event
	// Ищем событие с нужным типом и состоянием "активное"
	err := r.collection.FindOne(ctx, bson.M{"type": eventType, "state": Active}).Decode(&event)
	if err == mongo.ErrNoDocuments {
		// Если ничего не нашлось — это нормально, просто вернём nil
		return nil, nil
	}
	if err != nil {
		// Если произошла другая ошибка — вернём её
		return nil, err
	}
	return &event, nil
}

// Create создаёт новое событие в базе данных
// Автоматически устанавливает состояние "активное" и время начала
func (r *EventRepository) Create(ctx context.Context, eventType string) (*Event, error) {
	// Создаём событие со всеми необходимыми полями
	event := &Event{
		Type:      eventType,
		State:     Active,
		StartedAt: time.Now(),
	}
	// Сохраняем событие в базу данных
	result, err := r.collection.InsertOne(ctx, event)
	if err != nil {
		return nil, err
	}
	// Устанавливаем ID, который MongoDB автоматически создал при вставке
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		event.ID = oid
	} else {
		return nil, fmt.Errorf("неожиданный тип InsertedID: %T", result.InsertedID)
	}
	return event, nil
}

// Finish завершает активное событие указанного типа
// Находит его, меняет состояние на "завершено" и проставляет время окончания
// Если такого события нет, вернёт ошибку
func (r *EventRepository) Finish(ctx context.Context, eventType string) (*Event, error) {
	now := time.Now()
	// Ищем активное событие нужного типа
	filter := bson.M{"type": eventType, "state": Active}
	// Обновляем его: меняем состояние и проставляем время завершения
	update := bson.M{"$set": bson.M{"state": Finished, "finished_at": now}}
	// Настройки: вернуть обновлённый документ
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updated Event
	// Выполняем операцию поиска и обновления за один раз
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err == mongo.ErrNoDocuments {
		// Если события не нашлось — значит его и не было
		return nil, mongo.ErrNoDocuments
	}
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// List возвращает события из базы данных с учетом фильтров
// Параметры:
//   - offset: смещение от начала списка (0 = с самого начала)
//   - limit: максимальное количество событий (0 = без ограничения)
//   - eventType: фильтр по типу события (пустая строка = без фильтра)
//
// События отсортированы по времени начала в порядке убывания (descending)
func (r *EventRepository) List(ctx context.Context, offset int, limit int, eventType string) ([]Event, error) {
	// Строим фильтр для поиска
	filter := bson.M{}
	if eventType != "" {
		filter["type"] = eventType
	}

	// Настройки: сортировка по полю started_at по убыванию (descending)
	opts := options.Find().SetSort(bson.D{{Key: "started_at", Value: -1}})

	// Применяем offset и limit если они указаны
	if offset > 0 {
		opts.SetSkip(int64(offset))
	}
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	// Получаем события из базы с учетом фильтров
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	// Не забываем закрыть курсор, когда закончим с ним работать
	defer cursor.Close(ctx)

	var events []Event
	// Читаем все найденные события в массив
	err = cursor.All(ctx, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}
