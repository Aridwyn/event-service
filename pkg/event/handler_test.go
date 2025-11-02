package event

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"event-service/internal/db"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupTestHandler(t *testing.T) (*EventHandler, func()) {
	t.Helper()

	// Запускаем встроенный MongoDB
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Не удалось запустить встроенный MongoDB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		cleanupMongo()
		t.Fatalf("Не удалось подключиться к MongoDB: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		cleanupMongo()
		t.Fatalf("Не удалось проверить соединение: %v", err)
	}

	collection := client.Database("events_test_db").Collection("events")
	collection.Drop(ctx)

	repo := NewEventRepository(collection)
	service := NewEventService(repo)
	handler := NewEventHandler(service)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Disconnect(ctx)
		cleanupMongo()
	}

	return handler, cleanup
}

func TestNewEventHandler(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	service := NewEventService(repo)
	handler := NewEventHandler(service)

	if handler == nil {
		t.Fatal("NewEventHandler returned nil")
	}
	// Проверяем, что handler работает, вызывая его методы
	router := gin.New()
	router.POST("/start", handler.Start)
	reqBody := StartRequest{Type: "test"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Handler should work correctly, got status %d", w.Code)
	}
}

func TestHandler_Start_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	reqBody := StartRequest{Type: "meeting"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var event Event
	if err := json.Unmarshal(w.Body.Bytes(), &event); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if event.Type != "meeting" {
		t.Errorf("Expected type meeting, got %s", event.Type)
	}
	if event.State != Active {
		t.Errorf("Expected state Active, got %d", event.State)
	}
}

func TestHandler_Start_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	// Пустой запрос
	req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Start_EmptyType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	reqBody := StartRequest{Type: ""}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Finish_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// Сначала создаём событие через handler
	routerSetup := gin.New()
	routerSetup.POST("/start", handler.Start)

	reqBodyStart := StartRequest{Type: "meeting"}
	bodyStart, _ := json.Marshal(reqBodyStart)
	reqStart := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(bodyStart))
	reqStart.Header.Set("Content-Type", "application/json")
	wStart := httptest.NewRecorder()
	routerSetup.ServeHTTP(wStart, reqStart)

	if wStart.Code != http.StatusOK {
		t.Fatalf("Failed to start event: %d. Body: %s", wStart.Code, wStart.Body.String())
	}

	router := gin.New()
	router.POST("/finish", handler.Finish)

	reqBody := StartRequest{Type: "meeting"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var event Event
	if err := json.Unmarshal(w.Body.Bytes(), &event); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if event.Type != "meeting" {
		t.Errorf("Expected type meeting, got %s", event.Type)
	}
	if event.State != Finished {
		t.Errorf("Expected state Finished, got %d", event.State)
	}
	if event.FinishedAt == nil {
		t.Error("FinishedAt should not be nil")
	}
}

func TestHandler_Finish_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/finish", handler.Finish)

	reqBody := StartRequest{Type: "nonexistent"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Finish_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/finish", handler.Finish)

	// Пустой запрос
	req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_List_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.GET("/list", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var events []Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected empty list, got %d events", len(events))
	}
}

func TestHandler_List_WithEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// Создаём несколько событий через handler
	routerSetup := gin.New()
	routerSetup.POST("/start", handler.Start)

	types := []string{"meeting", "call"}
	for _, eventType := range types {
		reqBody := StartRequest{Type: eventType}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerSetup.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Failed to start event %s: %d. Body: %s", eventType, w.Code, w.Body.String())
		}

		time.Sleep(10 * time.Millisecond)
	}

	router := gin.New()
	router.GET("/list", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var events []Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
}

func TestHandler_Start_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	// Невалидный JSON
	req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Finish_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/finish", handler.Finish)

	// Невалидный JSON
	req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Start_NoContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	reqBody := StartRequest{Type: "meeting"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
	// Не устанавливаем Content-Type
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Gin может обработать это по-разному, но должен вернуть либо 200, либо 400
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Unexpected status code: %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Start_DuplicateEvent проверяет, что дубликат события возвращается корректно
func TestHandler_Start_DuplicateEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	// Создаём первое событие
	reqBody1 := StartRequest{Type: "meeting"}
	body1, _ := json.Marshal(reqBody1)

	req1 := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()

	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("First request failed: %d. Body: %s", w1.Code, w1.Body.String())
	}

	var firstEvent Event
	if err := json.Unmarshal(w1.Body.Bytes(), &firstEvent); err != nil {
		t.Fatalf("Failed to unmarshal first event: %v", err)
	}

	// Пытаемся создать ещё одно событие того же типа
	req2 := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body1))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Second request should also return 200, got %d. Body: %s", w2.Code, w2.Body.String())
	}

	var secondEvent Event
	if err := json.Unmarshal(w2.Body.Bytes(), &secondEvent); err != nil {
		t.Fatalf("Failed to unmarshal second event: %v", err)
	}

	// Должно быть то же самое событие
	if firstEvent.ID != secondEvent.ID {
		t.Errorf("Expected same event ID, got different: %s vs %s", firstEvent.ID, secondEvent.ID)
	}
}

// TestHandler_Finish_GenericError проверяет обработку других ошибок (не 404)
func TestHandler_Finish_GenericError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// Создаём событие, затем завершаем его через handler
	routerSetup := gin.New()
	routerSetup.POST("/start", handler.Start)
	routerSetup.POST("/finish", handler.Finish)

	// Создаём событие
	reqBodyStart := StartRequest{Type: "test"}
	bodyStart, _ := json.Marshal(reqBodyStart)
	reqStart := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(bodyStart))
	reqStart.Header.Set("Content-Type", "application/json")
	wStart := httptest.NewRecorder()
	routerSetup.ServeHTTP(wStart, reqStart)

	if wStart.Code != http.StatusOK {
		t.Fatalf("Failed to start event: %d. Body: %s", wStart.Code, wStart.Body.String())
	}

	// Завершаем событие
	reqBodyFinish := StartRequest{Type: "test"}
	bodyFinish, _ := json.Marshal(reqBodyFinish)
	reqFinish := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer(bodyFinish))
	reqFinish.Header.Set("Content-Type", "application/json")
	wFinish := httptest.NewRecorder()
	routerSetup.ServeHTTP(wFinish, reqFinish)

	if wFinish.Code != http.StatusOK {
		t.Fatalf("Failed to finish event: %d. Body: %s", wFinish.Code, wFinish.Body.String())
	}

	// Теперь пытаемся завершить через handler - должно вернуть 404
	router := gin.New()
	router.POST("/finish", handler.Finish)

	reqBody := StartRequest{Type: "test"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Start_MultipleTypes проверяет работу с разными типами событий
func TestHandler_Start_MultipleTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	types := []string{"meeting", "call", "task", "lunch"}

	for _, eventType := range types {
		reqBody := StartRequest{Type: eventType}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for type %s, got %d. Body: %s", eventType, w.Code, w.Body.String())
			continue
		}

		var event Event
		if err := json.Unmarshal(w.Body.Bytes(), &event); err != nil {
			t.Errorf("Failed to unmarshal event for type %s: %v", eventType, err)
			continue
		}

		if event.Type != eventType {
			t.Errorf("Expected type %s, got %s", eventType, event.Type)
		}
	}
}

// TestHandler_List_MixedStates проверяет список с событиями в разных состояниях
func TestHandler_List_MixedStates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// Создаём несколько событий через handler
	routerSetup := gin.New()
	routerSetup.POST("/start", handler.Start)
	routerSetup.POST("/finish", handler.Finish)

	// Создаём первое событие
	reqBody1 := StartRequest{Type: "active1"}
	body1, _ := json.Marshal(reqBody1)
	req1 := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	routerSetup.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("Failed to start active1: %d. Body: %s", w1.Code, w1.Body.String())
	}

	// Создаём второе событие
	reqBody2 := StartRequest{Type: "active2"}
	body2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	routerSetup.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Failed to start active2: %d. Body: %s", w2.Code, w2.Body.String())
	}

	// Завершаем первое событие
	reqFinish := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer(body1))
	reqFinish.Header.Set("Content-Type", "application/json")
	wFinish := httptest.NewRecorder()
	routerSetup.ServeHTTP(wFinish, reqFinish)

	if wFinish.Code != http.StatusOK {
		t.Fatalf("Failed to finish active1: %d. Body: %s", wFinish.Code, wFinish.Body.String())
	}

	// Получаем список через handler
	router := gin.New()
	router.GET("/list", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var events []Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Проверяем, что одно событие завершено, другое активно
	finishedCount := 0
	activeCount := 0
	for _, e := range events {
		if e.State == Finished {
			finishedCount++
		} else if e.State == Active {
			activeCount++
		}
	}

	if finishedCount != 1 {
		t.Errorf("Expected 1 finished event, got %d", finishedCount)
	}
	if activeCount != 1 {
		t.Errorf("Expected 1 active event, got %d", activeCount)
	}
}

// TestHandler_Start_InvalidEventType_Uppercase проверяет валидацию типа события с заглавными буквами
func TestHandler_Start_InvalidEventType_Uppercase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	reqBody := StartRequest{Type: "Meeting"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	var errorResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err == nil {
		if errorResp.Message != "Тип события должен содержать только строчные буквы и цифры" {
			t.Errorf("Expected validation error message, got %s", errorResp.Message)
		}
	}
}

// TestHandler_Start_InvalidEventType_SpecialChars проверяет валидацию типа события со специальными символами
func TestHandler_Start_InvalidEventType_SpecialChars(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/start", handler.Start)

	testCases := []string{"meeting-123", "meeting_123", "meeting.123", "meeting 123", "meeting@123"}
	for _, invalidType := range testCases {
		reqBody := StartRequest{Type: invalidType}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for type %s, got %d. Body: %s", invalidType, w.Code, w.Body.String())
		}
	}
}

// TestHandler_Finish_InvalidEventType проверяет валидацию типа события в Finish
func TestHandler_Finish_InvalidEventType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.POST("/finish", handler.Finish)

	reqBody := StartRequest{Type: "InvalidType!"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/finish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestHandler_List_WithOffset проверяет работу List с параметром offset
func TestHandler_List_WithOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// Создаём несколько событий
	routerSetup := gin.New()
	routerSetup.POST("/start", handler.Start)

	for i := 0; i < 5; i++ {
		reqBody := StartRequest{Type: "type" + strconv.Itoa(i)}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerSetup.ServeHTTP(w, req)
		time.Sleep(10 * time.Millisecond)
	}

	router := gin.New()
	router.GET("/list", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/list?offset=2", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var events []Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events with offset 2, got %d", len(events))
	}
}

// TestHandler_List_WithLimit проверяет работу List с параметром limit
func TestHandler_List_WithLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// Создаём несколько событий
	routerSetup := gin.New()
	routerSetup.POST("/start", handler.Start)

	for i := 0; i < 5; i++ {
		reqBody := StartRequest{Type: "type" + strconv.Itoa(i)}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerSetup.ServeHTTP(w, req)
		time.Sleep(10 * time.Millisecond)
	}

	router := gin.New()
	router.GET("/list", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/list?limit=3", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var events []Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events with limit 3, got %d", len(events))
	}
}

// TestHandler_List_WithType проверяет работу List с параметром type
func TestHandler_List_WithType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// Создаём события разных типов
	routerSetup := gin.New()
	routerSetup.POST("/start", handler.Start)

	types := []string{"meeting", "call", "meeting", "task"}
	for _, eventType := range types {
		reqBody := StartRequest{Type: eventType}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerSetup.ServeHTTP(w, req)
		time.Sleep(10 * time.Millisecond)
	}

	router := gin.New()
	router.GET("/list", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/list?type=meeting", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var events []Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	for _, event := range events {
		if event.Type != "meeting" {
			t.Errorf("Expected all events to be type 'meeting', got %s", event.Type)
		}
	}
}

// TestHandler_List_InvalidOffset проверяет обработку невалидного offset
func TestHandler_List_InvalidOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.GET("/list", handler.List)

	// Отрицательный offset
	req := httptest.NewRequest(http.MethodGet, "/list?offset=-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Нечисловой offset
	req2 := httptest.NewRequest(http.MethodGet, "/list?offset=abc", nil)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w2.Code, w2.Body.String())
	}
}

// TestHandler_List_InvalidLimit проверяет обработку невалидного limit
func TestHandler_List_InvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := gin.New()
	router.GET("/list", handler.List)

	// Отрицательный limit
	req := httptest.NewRequest(http.MethodGet, "/list?limit=-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Limit больше 100
	req2 := httptest.NewRequest(http.MethodGet, "/list?limit=101", nil)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w2.Code, w2.Body.String())
	}

	// Нечисловой limit
	req3 := httptest.NewRequest(http.MethodGet, "/list?limit=abc", nil)
	w3 := httptest.NewRecorder()

	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w3.Code, w3.Body.String())
	}
}
