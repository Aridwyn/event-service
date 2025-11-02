package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-service/internal/db"
	"event-service/pkg/event"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// getMongoURI получает URI для подключения к MongoDB
// Если установлена переменная окружения MONGO_URI — использует её
// Если нет — запускает встроенный MongoDB
func getMongoURI() (string, func(), error) {
	externalURI := os.Getenv("MONGO_URI")
	if externalURI != "" {
		log.Println("Используется внешний MongoDB из переменной окружения MONGO_URI")
		return externalURI, func() {}, nil
	}

	// Запускаем встроенный MongoDB
	log.Println("Запуск встроенного MongoDB...")
	mongoURI, cleanup, err := db.StartEmbeddedMongo()
	if err != nil {
		return "", nil, err
	}
	return mongoURI, cleanup, nil
}

// connectToMongoDB подключается к MongoDB
func connectToMongoDB(mongoURI string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}

	// Проверяем, что подключение действительно работает
	err = client.Ping(ctx, nil)
	if err != nil {
		client.Disconnect(ctx)
		return nil, err
	}

	return client, nil
}

// setupRouter настраивает и возвращает HTTP роутер
func setupRouter(handler *event.EventHandler) *gin.Engine {
	r := gin.Default()

	// Группируем все маршруты под префиксом /v1
	v1 := r.Group("/v1")
	{
		// GET /v1 — получить список всех событий, отсортированных по времени начала
		v1.GET("", handler.List)

		// POST /v1/start — создать новое событие указанного типа
		// Если активное событие этого типа уже есть — ничего не делает, возвращает существующее
		v1.POST("/start", handler.Start)

		// POST /v1/finish — завершить активное событие указанного типа
		// Если такого события нет — вернёт 404
		v1.POST("/finish", handler.Finish)
	}

	return r
}

// cleanupConnection закрывает соединение с MongoDB
func cleanupConnection(client *mongo.Client) {
	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer disconnectCancel()

	if err := client.Disconnect(disconnectCtx); err != nil {
		log.Printf("Ошибка при отключении от MongoDB: %v", err)
	}
}

// setupGracefulShutdown настраивает graceful shutdown для MongoDB клиента
func setupGracefulShutdown(client *mongo.Client, mongoCleanup func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Ждём сигнала о завершении работы
		<-sigChan
		log.Println("Получен сигнал завершения, закрываем соединение с MongoDB...")

		// Закрываем соединение с базой данных
		cleanupConnection(client)
		log.Println("Соединение с MongoDB успешно закрыто")

		// Если использовался встроенный MongoDB — останавливаем его
		mongoCleanup()

		os.Exit(0)
	}()
}

func main() {
	// Получаем URI для подключения к MongoDB
	mongoURI, mongoCleanup, err := getMongoURI()
	if err != nil {
		log.Fatal("Не удалось запустить встроенный MongoDB:", err)
	}
	defer mongoCleanup()

	// Подключаемся к MongoDB
	client, err := connectToMongoDB(mongoURI)
	if err != nil {
		log.Fatal("Не удалось подключиться к MongoDB:", err)
	}

	// Настраиваем graceful shutdown
	setupGracefulShutdown(client, mongoCleanup)

	// Закроем соединение с базой, когда программа завершится
	// Это подстраховка на случай, если graceful shutdown не сработает
	defer cleanupConnection(client)

	// Выбираем базу данных и коллекцию, с которой будем работать
	// База называется "events_db", коллекция — "events"
	collection := client.Database("events_db").Collection("events")

	// Создаём репозиторий — он будет работать с базой данных напрямую
	repo := event.NewEventRepository(collection)

	// Создаём сервис — он содержит бизнес-логику (проверки, правила и т.д.)
	service := event.NewEventService(repo)

	// Создаём обработчик HTTP-запросов — он будет принимать запросы от клиентов
	handler := event.NewEventHandler(service)

	// Настраиваем роутер
	r := setupRouter(handler)

	// Запускаем сервер
	startServer(r)
}

// logServerInfo пишет информацию о сервере в лог
func logServerInfo() {
	log.Println("Сервер запущен на порту :8080")
	log.Println("Доступные эндпоинты:")
	log.Println("  GET  /v1 — получить список всех событий")
	log.Println("  POST /v1/start — создать новое событие")
	log.Println("  POST /v1/finish — завершить событие")
}

// startServer запускает HTTP-сервер и пишет логи
func startServer(r *gin.Engine) {
	// Пишем в лог, что сервер запускается
	logServerInfo()

	// Запускаем HTTP-сервер на порту 8080
	// Если что-то пойдёт не так (например, порт уже занят) — программа завершится с ошибкой
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Не удалось запустить сервер:", err)
	}
}
