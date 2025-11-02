package event

import (
	"context"
	"testing"
	"time"

	"event-service/internal/db"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupTestRepo(t *testing.T) (*EventRepository, func()) {
	t.Helper()

	// Запускаем встроенный MongoDB
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Не удалось запустить встроенный MongoDB: %v", err)
	}

	// Подключаемся к MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		cleanupMongo()
		t.Fatalf("Не удалось подключиться к MongoDB: %v", err)
	}

	// Проверяем подключение
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		cleanupMongo()
		t.Fatalf("Не удалось проверить соединение: %v", err)
	}

	// Создаём тестовую коллекцию
	collection := client.Database("events_test_db").Collection("events")
	collection.Drop(ctx) // Очищаем перед тестами

	repo := NewEventRepository(collection)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Disconnect(ctx)
		cleanupMongo()
	}

	return repo, cleanup
}

func TestNewEventService(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)

	if service == nil {
		t.Fatal("NewEventService returned nil")
	}
	if service.repo != repo {
		t.Error("Service repository was not set correctly")
	}
}

func TestEventService_Start_NoActiveEvent(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()
	eventType := "meeting"

	// Создаём новое событие
	event, err := service.Start(ctx, eventType)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if event == nil {
		t.Fatal("Start returned nil event")
	}
	if event.Type != eventType {
		t.Errorf("Expected type %s, got %s", eventType, event.Type)
	}
	if event.State != Active {
		t.Errorf("Expected state Active, got %d", event.State)
	}
	if event.ID.IsZero() {
		t.Error("Event ID should not be zero")
	}
	if event.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

func TestEventService_Start_ActiveEventExists(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()
	eventType := "meeting"

	// Создаём первое событие
	firstEvent, err := service.Start(ctx, eventType)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	firstStartedAt := firstEvent.StartedAt

	// Пытаемся создать ещё одно событие того же типа
	secondEvent, err := service.Start(ctx, eventType)
	if err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}

	// Должно вернуться то же самое событие
	if secondEvent.ID != firstEvent.ID {
		t.Errorf("Expected same event ID, got different: %s vs %s", firstEvent.ID, secondEvent.ID)
	}
	// Проверяем, что StartedAt различается не более чем на 100мс (MongoDB может иметь небольшую задержку при чтении)
	timeDiff := secondEvent.StartedAt.Sub(firstStartedAt)
	if timeDiff > 100*time.Millisecond || timeDiff < -100*time.Millisecond {
		t.Errorf("Expected same StartedAt, got different")
	}
	if secondEvent.State != Active {
		t.Errorf("Expected state Active, got %d", secondEvent.State)
	}
}

func TestEventService_Finish_Success(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()
	eventType := "meeting"

	// Создаём событие
	startEvent, err := service.Start(ctx, eventType)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Завершаем событие
	finishedEvent, err := service.Finish(ctx, eventType)
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	if finishedEvent == nil {
		t.Fatal("Finish returned nil event")
	}
	if finishedEvent.ID != startEvent.ID {
		t.Errorf("Expected same event ID, got different")
	}
	if finishedEvent.State != Finished {
		t.Errorf("Expected state Finished, got %d", finishedEvent.State)
	}
	if finishedEvent.FinishedAt == nil {
		t.Error("FinishedAt should not be nil")
	}
}

func TestEventService_Finish_NotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()
	eventType := "nonexistent"

	// Пытаемся завершить несуществующее событие
	event, err := service.Finish(ctx, eventType)
	if err == nil {
		t.Fatal("Expected error when finishing non-existent event")
	}
	if err != mongo.ErrNoDocuments {
		t.Errorf("Expected mongo.ErrNoDocuments, got %v", err)
	}
	if event != nil {
		t.Error("Expected nil event when error occurs")
	}
}

func TestEventService_List_Empty(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()

	// Получаем список событий (должен быть пустым)
	events, err := service.List(ctx, 0, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected empty list, got %d events", len(events))
	}
}

func TestEventService_List_WithEvents(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()

	// Создаём несколько событий
	firstType := "meeting"
	secondType := "call"
	thirdType := "task"

	_, err := service.Start(ctx, firstType)
	if err != nil {
		t.Fatalf("Start first failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Небольшая задержка для разных времён

	_, err = service.Start(ctx, secondType)
	if err != nil {
		t.Fatalf("Start second failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = service.Start(ctx, thirdType)
	if err != nil {
		t.Fatalf("Start third failed: %v", err)
	}

	// Получаем список событий
	events, err := service.List(ctx, 0, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	// Проверяем сортировку по started_at (должна быть descending - от поздних к ранним)
	for i := 1; i < len(events); i++ {
		if events[i].StartedAt.After(events[i-1].StartedAt) {
			t.Error("Events should be sorted by started_at in descending order")
		}
	}
}

func TestEventService_Start_ErrorHandling(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)

	// Используем отменённый контекст для проверки обработки ошибок
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Отменяем контекст сразу

	// Попытка создать событие с отменённым контекстом должна вернуть ошибку
	event, err := service.Start(ctx, "test")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	if event != nil {
		t.Error("Expected nil event when error occurs")
	}
}

func TestEventService_Finish_DifferentTypes(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()

	// Создаём два события разных типов
	_, err := service.Start(ctx, "meeting")
	if err != nil {
		t.Fatalf("Start meeting failed: %v", err)
	}

	_, err = service.Start(ctx, "call")
	if err != nil {
		t.Fatalf("Start call failed: %v", err)
	}

	// Завершаем только одно
	finished, err := service.Finish(ctx, "meeting")
	if err != nil {
		t.Fatalf("Finish meeting failed: %v", err)
	}

	if finished.Type != "meeting" {
		t.Errorf("Expected type meeting, got %s", finished.Type)
	}

	// Проверяем, что call всё ещё активен
	events, err := service.List(ctx, 0, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	callFound := false
	for _, e := range events {
		if e.Type == "call" && e.State == Active {
			callFound = true
			break
		}
	}

	if !callFound {
		t.Error("Call event should still be active")
	}
}

func TestEventService_Start_WithRepositoryError(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)

	// Используем отменённый контекст для создания ошибки на уровне репозитория
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Попытка создать событие с отменённым контекстом
	event, err := service.Start(cancelledCtx, "test")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	if event != nil {
		t.Error("Expected nil event when error occurs")
	}
}

func TestEventService_Finish_WithGenericError(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)

	// Используем отменённый контекст для создания ошибки
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Попытка завершить с отменённым контекстом должна вернуть ошибку
	event, err := service.Finish(cancelledCtx, "test")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	// Ошибка может быть не mongo.ErrNoDocuments, а ошибка контекста
	if event != nil {
		t.Error("Expected nil event when error occurs")
	}
}

func TestEventService_List_WithContextError(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)

	// Используем отменённый контекст
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Попытка получить список с отменённым контекстом
	events, err := service.List(cancelledCtx, 0, 0, "")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
	if events != nil {
		t.Error("Expected nil events when error occurs")
	}
}

func TestEventService_List_WithOffset(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()

	// Создаём несколько событий разных типов (чтобы каждое было уникальным)
	for i := 0; i < 5; i++ {
		eventType := "type" + string(rune('0'+i))
		_, err := service.Start(ctx, eventType)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с offset
	events, err := service.List(ctx, 2, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events with offset 2, got %d", len(events))
	}
}

func TestEventService_List_WithLimit(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()

	// Создаём несколько событий разных типов
	for i := 0; i < 5; i++ {
		eventType := "type" + string(rune('0'+i))
		_, err := service.Start(ctx, eventType)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с limit
	events, err := service.List(ctx, 0, 3, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events with limit 3, got %d", len(events))
	}
}

func TestEventService_List_WithTypeFilter(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()

	// Создаём события разных типов
	types := []string{"meeting", "call", "meeting", "task"}
	for _, eventType := range types {
		// Сначала создаём событие, потом завершаем его, чтобы можно было создать новое того же типа
		_, err := service.Start(ctx, eventType)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		_, err = service.Finish(ctx, eventType)
		if err != nil {
			t.Fatalf("Finish failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с фильтром по типу
	events, err := service.List(ctx, 0, 0, "meeting")
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

func TestEventService_List_WithOffsetAndLimit(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	ctx := context.Background()

	// Создаём события разных типов (чтобы создать 10 уникальных событий)
	eventTypes := []string{"type0", "type1", "type2", "type3", "type4", "type5", "type6", "type7", "type8", "type9"}
	for _, eventType := range eventTypes {
		_, err := service.Start(ctx, eventType)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Получаем список с offset и limit
	events, err := service.List(ctx, 3, 5, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("Expected 5 events with offset 3 and limit 5, got %d", len(events))
	}
}
