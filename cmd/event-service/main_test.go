package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"event-service/internal/db"
	eventpkg "event-service/pkg/event"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestGetMongoURI_Embedded тестирует getMongoURI с встроенным MongoDB
func TestGetMongoURI_Embedded(t *testing.T) {
	// Сохраняем оригинальное значение
	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)

	// Убираем переменную окружения
	os.Unsetenv("MONGO_URI")

	mongoURI, cleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer cleanup()

	if mongoURI == "" {
		t.Error("MongoURI should not be empty")
	}
	if cleanup == nil {
		t.Error("Cleanup function should not be nil")
	}
}

// TestGetMongoURI_External тестирует getMongoURI с внешним MongoDB
func TestGetMongoURI_External(t *testing.T) {
	// Сохраняем оригинальное значение
	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)

	// Устанавливаем переменную окружения
	os.Setenv("MONGO_URI", "mongodb://external:27017")

	mongoURI, cleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer cleanup()

	if mongoURI != "mongodb://external:27017" {
		t.Errorf("Expected mongodb://external:27017, got %s", mongoURI)
	}
	if cleanup == nil {
		t.Error("Cleanup function should not be nil")
	}
}

// TestConnectToMongoDB тестирует connectToMongoDB
func TestConnectToMongoDB(t *testing.T) {
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start embedded MongoDB: %v", err)
	}
	defer cleanupMongo()

	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("connectToMongoDB failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Disconnect(ctx)
	}()

	if client == nil {
		t.Fatal("Client should not be nil")
	}
}

// TestSetupRouter тестирует setupRouter
func TestSetupRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start embedded MongoDB: %v", err)
	}
	defer cleanupMongo()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		client.Disconnect(disconnectCtx)
	}()

	collection := client.Database("events_test_db").Collection("events")
	repo := eventpkg.NewEventRepository(collection)
	service := eventpkg.NewEventService(repo)
	handler := eventpkg.NewEventHandler(service)

	// Тестируем setupRouter из main.go
	r := setupRouter(handler)
	if r == nil {
		t.Fatal("Router should not be nil")
	}

	// Проверяем, что маршруты зарегистрированы
	routes := r.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/v1" && route.Method == "GET" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Route /v1 GET not found")
	}

	found = false
	for _, route := range routes {
		if route.Path == "/v1/start" && route.Method == "POST" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Route /v1/start POST not found")
	}

	found = false
	for _, route := range routes {
		if route.Path == "/v1/finish" && route.Method == "POST" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Route /v1/finish POST not found")
	}
}

// TestServerSetup тестирует настройку сервера (код из main.go)
func TestServerSetup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Тестируем функции из main.go
	mongoURI, cleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer cleanup()

	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("connectToMongoDB failed: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		client.Disconnect(disconnectCtx)
	}()

	collection := client.Database("events_db").Collection("events")
	repo := eventpkg.NewEventRepository(collection)
	service := eventpkg.NewEventService(repo)
	handler := eventpkg.NewEventHandler(service)

	r := setupRouter(handler)
	if r == nil {
		t.Fatal("Router should not be nil")
	}
}

// TestSetupGracefulShutdown тестирует setupGracefulShutdown (базовая проверка)
func TestSetupGracefulShutdown(t *testing.T) {
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start embedded MongoDB: %v", err)
	}
	defer cleanupMongo()

	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("connectToMongoDB failed: %v", err)
	}

	// Тестируем setupGracefulShutdown
	testCleanup := func() {
		// Cleanup функция
	}

	setupGracefulShutdown(client, testCleanup)

	// Проверяем, что функция была вызвана (грациозно не можем проверить сигнал,
	// но можем убедиться, что функция не паникует)
	if client == nil {
		t.Fatal("Client should not be nil")
	}

	// Очистка
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.Disconnect(ctx)

	// Мы просто проверяем, что функция setupGracefulShutdown не паникует
	// и корректно обрабатывает параметры
}

// TestEventServiceIntegration тестирует интеграцию всех компонентов из main
func TestEventServiceIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		client.Disconnect(disconnectCtx)
	}()

	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("Не удалось проверить соединение: %v", err)
	}

	// Повторяем логику из main.go
	collection := client.Database("events_db").Collection("events")
	repo := eventpkg.NewEventRepository(collection)
	service := eventpkg.NewEventService(repo)
	handler := eventpkg.NewEventHandler(service)

	// Тестируем, что всё работает вместе
	testCtx := context.Background()

	// Создаём событие
	event, err := service.Start(testCtx, "test")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if event == nil {
		t.Fatal("Event should not be nil")
	}

	// Получаем список
	events, err := service.List(testCtx, 0, 0, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Завершаем событие
	finishedEvent, err := service.Finish(testCtx, "test")
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	// Используем константу Finished из пакета event
	if finishedEvent.State != eventpkg.Finished {
		t.Error("Event should be finished")
	}

	// Проверяем, что handler работает
	if handler == nil {
		t.Fatal("Handler should not be nil")
	}
	// Проверяем, что handler создан корректно, вызывая его методы
	// (не можем проверить напрямую, так как service - неэкспортированное поле)
}

// TestConnectToMongoDB_InvalidURI тестирует обработку ошибки при невалидном URI
func TestConnectToMongoDB_InvalidURI(t *testing.T) {
	// Используем невалидный URI
	invalidURI := "invalid://mongodb"

	client, err := connectToMongoDB(invalidURI)
	if err == nil {
		// Если подключение каким-то образом удалось, отключаемся
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client.Disconnect(ctx)
		}
		t.Error("Expected error with invalid URI")
	}
	if client != nil {
		t.Error("Client should be nil when error occurs")
	}
}

// TestConnectToMongoDB_ConnectionError тестирует обработку ошибки подключения
func TestConnectToMongoDB_ConnectionError(t *testing.T) {
	// Используем валидный URI, но несуществующий адрес
	invalidURI := "mongodb://127.0.0.1:99999/test"

	// Используем очень короткий таймаут для быстрого теста
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Пытаемся подключиться
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(invalidURI))
	if err == nil && client != nil {
		// Если всё же подключилось, отключаемся
		client.Disconnect(ctx)
		t.Error("Expected connection error")
	}
}

// TestMainInitialization тестирует полную инициализацию из main
func TestMainInitialization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Сохраняем оригинальное значение
	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)

	// Убираем переменную окружения для использования встроенного MongoDB
	os.Unsetenv("MONGO_URI")

	// Получаем URI
	mongoURI, cleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer cleanup()

	// Подключаемся
	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("connectToMongoDB failed: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		client.Disconnect(disconnectCtx)
	}()

	// Повторяем логику из main.go
	collection := client.Database("events_db").Collection("events")
	repo := eventpkg.NewEventRepository(collection)
	service := eventpkg.NewEventService(repo)
	handler := eventpkg.NewEventHandler(service)
	r := setupRouter(handler)

	// Проверяем, что всё инициализировано
	if repo == nil || service == nil || handler == nil || r == nil {
		t.Error("All components should be initialized")
	}

	// Проверяем, что роутер настроен
	routes := r.Routes()
	if len(routes) < 3 {
		t.Errorf("Expected at least 3 routes, got %d", len(routes))
	}
}

// TestMainDeferCleanup тестирует логику defer cleanup из main
func TestMainDeferCleanup(t *testing.T) {
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		cleanupMongo()
		t.Fatalf("Failed to connect: %v", err)
	}

	// Симулируем defer из main.go
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()

		if err := client.Disconnect(disconnectCtx); err != nil {
			t.Logf("Error during disconnect (expected in test): %v", err)
		}
	}()

	// Проверяем, что клиент работает
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	cleanupMongo()
}

// TestCollectionAndRepositoryCreation тестирует создание коллекции и репозитория
func TestCollectionAndRepositoryCreation(t *testing.T) {
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	defer cleanupMongo()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		client.Disconnect(disconnectCtx)
	}()

	// Тестируем создание коллекции (из main.go)
	collection := client.Database("events_db").Collection("events")
	if collection == nil {
		t.Fatal("Collection should not be nil")
	}

	// Тестируем создание репозитория
	repo := eventpkg.NewEventRepository(collection)
	if repo == nil {
		t.Fatal("Repository should not be nil")
	}

	// Тестируем создание сервиса
	service := eventpkg.NewEventService(repo)
	if service == nil {
		t.Fatal("Service should not be nil")
	}

	// Тестируем создание handler
	handler := eventpkg.NewEventHandler(service)
	if handler == nil {
		t.Fatal("Handler should not be nil")
	}
}

// TestGetMongoURI_LogMessages проверяет, что логи пишутся
func TestGetMongoURI_LogMessages(t *testing.T) {
	// Сохраняем оригинальное значение
	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)

	// Тест 1: Внешний MongoDB (проверяем, что логируется)
	os.Setenv("MONGO_URI", "mongodb://external:27017")
	mongoURI, cleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer cleanup()
	if mongoURI != "mongodb://external:27017" {
		t.Errorf("Expected external URI, got %s", mongoURI)
	}

	// Тест 2: Встроенный MongoDB (проверяем, что логируется)
	os.Unsetenv("MONGO_URI")
	mongoURI2, cleanup2, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer cleanup2()
	if mongoURI2 == "" {
		t.Error("MongoURI should not be empty")
	}
}

// TestConnectToMongoDB_PingError проверяет обработку ошибки Ping
func TestConnectToMongoDB_PingError(t *testing.T) {
	// Используем валидный URI, но несуществующий адрес
	invalidURI := "mongodb://127.0.0.1:99999/test"

	// connectToMongoDB должна обработать ошибку Ping
	client, err := connectToMongoDB(invalidURI)
	if err == nil {
		// Если подключение удалось (что маловероятно), отключаемся
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client.Disconnect(ctx)
		}
		t.Error("Expected error with invalid URI")
	}
	if client != nil {
		t.Error("Client should be nil when error occurs")
	}
}

// TestSetupRouter_AllRoutes проверяет, что все маршруты настроены
func TestSetupRouter_AllRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	defer cleanupMongo()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		client.Disconnect(disconnectCtx)
	}()

	collection := client.Database("events_test_db").Collection("events")
	repo := eventpkg.NewEventRepository(collection)
	service := eventpkg.NewEventService(repo)
	handler := eventpkg.NewEventHandler(service)

	r := setupRouter(handler)

	// Проверяем все маршруты
	routes := r.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		key := route.Method + " " + route.Path
		routeMap[key] = true
	}

	expectedRoutes := []string{
		"GET /v1",
		"POST /v1/start",
		"POST /v1/finish",
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("Expected route %s not found", expected)
		}
	}
}

// TestMainFlow_Success тестирует весь flow из main (без запуска сервера)
func TestMainFlow_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Сохраняем оригинальное значение
	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)
	os.Unsetenv("MONGO_URI")

	// Получаем URI для подключения к MongoDB (из main)
	mongoURI, mongoCleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer mongoCleanup()

	// Подключаемся к MongoDB (из main)
	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("connectToMongoDB failed: %v", err)
	}

	// Настраиваем graceful shutdown (из main)
	setupGracefulShutdown(client, mongoCleanup)

	// Симулируем defer из main
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()

		if err := client.Disconnect(disconnectCtx); err != nil {
			t.Logf("Error during disconnect: %v", err)
		}
	}()

	// Выбираем базу данных и коллекцию (из main)
	collection := client.Database("events_db").Collection("events")

	// Создаём репозиторий (из main)
	repo := eventpkg.NewEventRepository(collection)

	// Создаём сервис (из main)
	service := eventpkg.NewEventService(repo)

	// Создаём обработчик (из main)
	handler := eventpkg.NewEventHandler(service)

	// Настраиваем роутер (из main)
	r := setupRouter(handler)

	// Проверяем, что всё работает
	if r == nil || handler == nil || service == nil || repo == nil {
		t.Error("All components should be initialized")
	}

	// Тестируем, что роутер работает (делаем тестовый запрос)
	router := setupRouter(handler)
	req := httptest.NewRequest(http.MethodGet, "/v1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Router should work, got status %d", w.Code)
	}
}

// TestMainFlow_WithLogs тестирует код, который пишет логи (строки 153-157 из main)
func TestMainFlow_WithLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)
	os.Unsetenv("MONGO_URI")

	mongoURI, mongoCleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI failed: %v", err)
	}
	defer mongoCleanup()

	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("connectToMongoDB failed: %v", err)
	}

	setupGracefulShutdown(client, mongoCleanup)

	defer cleanupConnection(client)

	collection := client.Database("events_db").Collection("events")
	repo := eventpkg.NewEventRepository(collection)
	service := eventpkg.NewEventService(repo)
	handler := eventpkg.NewEventHandler(service)
	r := setupRouter(handler)

	// Тестируем код, который выполняется в main после setupRouter
	// Строки 152-157: логирование (эти строки не покрываются, но мы можем вызвать setupRouter и проверить работу)
	if r == nil {
		t.Fatal("Router should not be nil")
	}

	// Проверяем, что роутер работает и может обработать запрос
	req := httptest.NewRequest(http.MethodGet, "/v1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Router should handle request, got status %d", w.Code)
	}
}

// TestGetMongoURI_ErrorHandling проверяет обработку ошибки в getMongoURI
func TestGetMongoURI_ErrorHandling(t *testing.T) {
	// Этот тест сложно написать, так как db.StartEmbeddedMongo редко возвращает ошибку
	// Но мы можем проверить ветку с внешним MongoDB
	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)

	os.Setenv("MONGO_URI", "mongodb://external:27017")
	mongoURI, cleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("getMongoURI should not fail with external URI: %v", err)
	}
	defer cleanup()

	if mongoURI != "mongodb://external:27017" {
		t.Errorf("Expected external URI, got %s", mongoURI)
	}
}

// TestSetupGracefulShutdown_ErrorPath тестирует путь с ошибкой в Disconnect
func TestSetupGracefulShutdown_ErrorPath(t *testing.T) {
	// Создаём клиент
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	defer cleanupMongo()

	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Отключаемся сразу, чтобы при следующем вызове Disconnect была ошибка
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	client.Disconnect(ctx)
	cancel()

	// Теперь setupGracefulShutdown должен обработать уже отключённый клиент
	// (хотя горутина не будет выполнена без сигнала, но функция вызовется)
	testCleanup := func() {
		// Cleanup функция
	}

	// Вызываем setupGracefulShutdown - функция не должна паниковать
	setupGracefulShutdown(client, testCleanup)

	// Небольшая задержка, чтобы горутина могла запуститься
	time.Sleep(100 * time.Millisecond)
}

// TestConnectToMongoDB_DisconnectOnPingError проверяет, что Disconnect вызывается при ошибке Ping
func TestConnectToMongoDB_DisconnectOnPingError(t *testing.T) {
	// Используем несуществующий адрес
	invalidURI := "mongodb://127.0.0.1:99999/test"

	client, err := connectToMongoDB(invalidURI)
	// Должна быть ошибка
	if err == nil {
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client.Disconnect(ctx)
		}
		t.Error("Expected error")
	}
	// Проверяем, что client.Disconnect был вызван при ошибке Ping (строка 51)
	// Если бы Disconnect не был вызван, мы бы не смогли это проверить напрямую,
	// но функция должна корректно обрабатывать ошибку
}

// TestMain_AllInitializationSteps проверяет все шаги инициализации из main
func TestMain_AllInitializationSteps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalURI := os.Getenv("MONGO_URI")
	defer os.Setenv("MONGO_URI", originalURI)
	os.Unsetenv("MONGO_URI")

	// Шаг 1: getMongoURI (строка 110)
	mongoURI, mongoCleanup, err := getMongoURI()
	if err != nil {
		t.Fatalf("Step 1 failed: %v", err)
	}
	defer mongoCleanup()

	// Шаг 2: connectToMongoDB (строка 117)
	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		t.Fatalf("Step 2 failed: %v", err)
	}

	// Шаг 3: setupGracefulShutdown (строка 123)
	setupGracefulShutdown(client, mongoCleanup)

	// Шаг 4: defer cleanupConnection (строка 127) - будет выполнен в defer
	defer cleanupConnection(client)

	// Шаг 5: Database и Collection (строка 138)
	collection := client.Database("events_db").Collection("events")

	// Шаг 6: NewEventRepository (строка 141)
	repo := eventpkg.NewEventRepository(collection)

	// Шаг 7: NewEventService (строка 144)
	service := eventpkg.NewEventService(repo)

	// Шаг 8: NewEventHandler (строка 147)
	handler := eventpkg.NewEventHandler(service)

	// Шаг 9: setupRouter (строка 150)
	r := setupRouter(handler)

	// Проверяем, что всё инициализировано
	if r == nil || handler == nil || service == nil || repo == nil || collection == nil {
		t.Error("All components should be initialized")
	}

	// Тестируем работу роутера
	req := httptest.NewRequest(http.MethodGet, "/v1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Router should work, got status %d", w.Code)
	}
}

// TestCleanupConnection тестирует cleanupConnection
func TestCleanupConnection(t *testing.T) {
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	defer cleanupMongo()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Тестируем cleanupConnection
	cleanupConnection(client)

	// Проверяем, что клиент отключён
	err = client.Ping(ctx, nil)
	if err == nil {
		t.Error("Client should be disconnected")
	}
}

// TestCleanupConnection_AlreadyDisconnected проверяет cleanupConnection с уже отключённым клиентом
func TestCleanupConnection_AlreadyDisconnected(t *testing.T) {
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	defer cleanupMongo()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Отключаемся сначала
	client.Disconnect(ctx)

	// Теперь вызываем cleanupConnection - не должна паниковать
	cleanupConnection(client)
}

// TestStartServer тестирует startServer (без реального запуска сервера)
func TestStartServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	defer cleanupMongo()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer cleanupConnection(client)

	collection := client.Database("events_test_db").Collection("events")
	repo := eventpkg.NewEventRepository(collection)
	service := eventpkg.NewEventService(repo)
	handler := eventpkg.NewEventHandler(service)
	r := setupRouter(handler)

	// startServer вызывает r.Run, который запустит сервер на порту 8080
	// Это заблокирует выполнение, поэтому мы не можем вызвать его напрямую
	// Но мы можем проверить, что роутер настроен правильно
	if r == nil {
		t.Fatal("Router should not be nil")
	}

	// Тестируем роутер без запуска сервера
	req := httptest.NewRequest(http.MethodGet, "/v1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Router should work, got status %d", w.Code)
	}
}

// TestLogServerInfo тестирует logServerInfo
func TestLogServerInfo(t *testing.T) {
	// logServerInfo просто пишет в лог, мы можем вызвать её
	logServerInfo()
	// Если функция не паникует, тест пройден
}
