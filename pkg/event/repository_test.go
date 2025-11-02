package event

import (
	"context"
	"testing"
	"time"

	"event-service/internal/db"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestNewEventRepository(t *testing.T) {
	// Создаём тестовую коллекцию
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Не удалось запустить встроенный MongoDB: %v", err)
	}
	defer cleanupMongo()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Не удалось подключиться к MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	collection := client.Database("events_test_db").Collection("events")
	repo := NewEventRepository(collection)

	if repo == nil {
		t.Fatal("NewEventRepository returned nil")
	}
	if repo.collection != collection {
		t.Error("Repository collection was not set correctly")
	}
}

func TestEventRepository_FindActive_NotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	event, err := repo.FindActive(ctx, "nonexistent")

	if err != nil {
		t.Errorf("FindActive should return nil error when not found, got: %v", err)
	}
	if event != nil {
		t.Error("FindActive should return nil event when not found")
	}
}

func TestEventRepository_FindActive_Found(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	eventType := "meeting"

	// Создаём событие
	created, err := repo.Create(ctx, eventType)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Ищем активное событие
	found, err := repo.FindActive(ctx, eventType)
	if err != nil {
		t.Fatalf("FindActive failed: %v", err)
	}

	if found == nil {
		t.Fatal("FindActive should return event")
	}
	if found.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, found.ID)
	}
	if found.Type != eventType {
		t.Errorf("Expected type %s, got %s", eventType, found.Type)
	}
	if found.State != Active {
		t.Errorf("Expected state Active, got %d", found.State)
	}
}

func TestEventRepository_Create(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	eventType := "meeting"

	event, err := repo.Create(ctx, eventType)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if event == nil {
		t.Fatal("Create returned nil event")
	}
	if event.ID.IsZero() {
		t.Error("Event ID should not be zero")
	}
	if event.Type != eventType {
		t.Errorf("Expected type %s, got %s", eventType, event.Type)
	}
	if event.State != Active {
		t.Errorf("Expected state Active, got %d", event.State)
	}
	if event.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

func TestEventRepository_Finish_Success(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	eventType := "meeting"

	// Создаём событие
	created, err := repo.Create(ctx, eventType)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Завершаем событие
	finished, err := repo.Finish(ctx, eventType)
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	if finished == nil {
		t.Fatal("Finish returned nil event")
	}
	if finished.ID != created.ID {
		t.Errorf("Expected same ID, got different")
	}
	if finished.State != Finished {
		t.Errorf("Expected state Finished, got %d", finished.State)
	}
	if finished.FinishedAt == nil {
		t.Error("FinishedAt should not be nil")
	}
}

func TestEventRepository_Finish_NotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	eventType := "nonexistent"

	event, err := repo.Finish(ctx, eventType)
	if err == nil {
		t.Fatal("Finish should return error when event not found")
	}
	if err != mongo.ErrNoDocuments {
		t.Errorf("Expected mongo.ErrNoDocuments, got %v", err)
	}
	if event != nil {
		t.Error("Finish should return nil event when error occurs")
	}
}

func TestEventRepository_Finish_AlreadyFinished(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	eventType := "meeting"

	// Создаём и завершаем событие
	_, err := repo.Create(ctx, eventType)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = repo.Finish(ctx, eventType)
	if err != nil {
		t.Fatalf("First Finish failed: %v", err)
	}

	// Пытаемся завершить уже завершённое событие
	event, err := repo.Finish(ctx, eventType)
	if err == nil {
		t.Fatal("Finish should return error when event already finished")
	}
	if err != mongo.ErrNoDocuments {
		t.Errorf("Expected mongo.ErrNoDocuments, got %v", err)
	}
	if event != nil {
		t.Error("Finish should return nil event when error occurs")
	}
}

func TestEventRepository_List_Empty(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	events, err := repo.List(ctx, 0, 0, "")

	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected empty list, got %d events", len(events))
	}
}

func TestEventRepository_List_WithEvents(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём несколько событий
	type1 := "meeting"
	type2 := "call"
	type3 := "task"

	_, err := repo.Create(ctx, type1)
	if err != nil {
		t.Fatalf("Create first failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = repo.Create(ctx, type2)
	if err != nil {
		t.Fatalf("Create second failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = repo.Create(ctx, type3)
	if err != nil {
		t.Fatalf("Create third failed: %v", err)
	}

	// Получаем список
	events, err := repo.List(ctx, 0, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	// Проверяем сортировку (должна быть descending - от поздних к ранним)
	for i := 1; i < len(events); i++ {
		if events[i].StartedAt.After(events[i-1].StartedAt) {
			t.Error("Events should be sorted by started_at in descending order")
		}
	}

	// Проверяем, что все события присутствуют
	types := make(map[string]bool)
	for _, e := range events {
		types[e.Type] = true
	}

	if !types[type1] || !types[type2] || !types[type3] {
		t.Error("Not all event types found in list")
	}
}

func TestEventRepository_List_SortedByStartedAt(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём события в разное время
	time1 := time.Now()
	_, err := repo.Create(ctx, "first")
	if err != nil {
		t.Fatalf("Create first failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, err = repo.Create(ctx, "second")
	if err != nil {
		t.Fatalf("Create second failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, err = repo.Create(ctx, "third")
	if err != nil {
		t.Fatalf("Create third failed: %v", err)
	}

	events, err := repo.List(ctx, 0, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	// Проверяем, что первое событие самое позднее (descending сортировка)
	if events[0].StartedAt.Before(time1) {
		t.Error("First event should not be before creation time")
	}

	// Проверяем сортировку (должна быть descending - от поздних к ранним)
	for i := 1; i < len(events); i++ {
		if events[i].StartedAt.After(events[i-1].StartedAt) {
			t.Errorf("Event %d started after event %d, but sorting should be descending", i, i-1)
		}
		if events[i].StartedAt.Equal(events[i-1].StartedAt) {
			t.Errorf("Events %d and %d have same started_at (unlikely)", i-1, i)
		}
	}
}

func TestEventRepository_Create_UniqueIDs(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём несколько событий
	ids := make(map[primitive.ObjectID]bool)
	for i := 0; i < 10; i++ {
		event, err := repo.Create(ctx, "test")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if ids[event.ID] {
			t.Errorf("Duplicate ID found: %s", event.ID)
		}
		ids[event.ID] = true
	}

	if len(ids) != 10 {
		t.Errorf("Expected 10 unique IDs, got %d", len(ids))
	}
}

func TestEventRepository_List_WithOffset(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём несколько событий
	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, "type"+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с offset
	events, err := repo.List(ctx, 2, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events with offset 2, got %d", len(events))
	}
}

func TestEventRepository_List_WithLimit(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём несколько событий
	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, "type"+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с limit
	events, err := repo.List(ctx, 0, 3, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events with limit 3, got %d", len(events))
	}
}

func TestEventRepository_List_WithTypeFilter(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём события разных типов
	types := []string{"meeting", "call", "meeting", "task"}
	for _, eventType := range types {
		_, err := repo.Create(ctx, eventType)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с фильтром по типу
	events, err := repo.List(ctx, 0, 0, "meeting")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events of type 'meeting', got %d", len(events))
	}

	for _, event := range events {
		if event.Type != "meeting" {
			t.Errorf("Expected all events to be type 'meeting', got %s", event.Type)
		}
	}
}

func TestEventRepository_List_WithOffsetAndLimit(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём несколько событий
	for i := 0; i < 10; i++ {
		_, err := repo.Create(ctx, "type")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с offset и limit
	events, err := repo.List(ctx, 3, 5, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("Expected 5 events with offset 3 and limit 5, got %d", len(events))
	}
}

func TestEventRepository_FindActive_WithError(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Используем отменённый контекст для проверки обработки ошибок
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event, err := repo.FindActive(ctx, "test")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	if event != nil {
		t.Error("Expected nil event when error occurs")
	}
}

func TestEventRepository_Create_WithError(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Используем отменённый контекст
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event, err := repo.Create(ctx, "test")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	if event != nil {
		t.Error("Expected nil event when error occurs")
	}
}

func TestEventRepository_Finish_WithError(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Используем отменённый контекст
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event, err := repo.Finish(ctx, "test")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	if event != nil {
		t.Error("Expected nil event when error occurs")
	}
}

func TestEventRepository_List_WithError(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Используем отменённый контекст
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events, err := repo.List(ctx, 0, 0, "")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	if events != nil {
		t.Error("Expected nil events when error occurs")
	}
}
