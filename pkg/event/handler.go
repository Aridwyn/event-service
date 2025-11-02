package event

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// StartRequest — это структура для запроса на создание/запуск события
// Используется для валидации данных, которые приходят от клиента
type StartRequest struct {
	// Type — тип события, который нужно создать или завершить
	// Поле обязательно — если его нет, вернём ошибку 400
	// Должен соответствовать pattern: ^[a-z0-9]+$
	Type string `json:"type" binding:"required"`
}

// ErrorResponse представляет ошибку в формате API согласно OpenAPI контракту
type ErrorResponse struct {
	Message string `json:"message"`
}

var typePattern = regexp.MustCompile(`^[a-z0-9]+$`)

// validateEventType проверяет, что тип события соответствует pattern '^[a-z0-9]+$'
func validateEventType(eventType string) bool {
	return typePattern.MatchString(eventType)
}

// EventHandler обрабатывает все HTTP-запросы, связанные с событиями
// Он получает запросы от клиента, проверяет их, вызывает сервис и отправляет ответы
type EventHandler struct {
	// service — это наш бизнес-слой, который знает, как работать с событиями
	service *EventService
}

// NewEventHandler создаёт новый обработчик HTTP-запросов
// Нужно просто передать ему сервис, который будет выполнять всю работу
func NewEventHandler(service *EventService) *EventHandler {
	return &EventHandler{service: service}
}

// Start обрабатывает запрос на запуск нового события
// Принимает JSON с полем "type" и создаёт новое событие, если активного ещё нет
func (h *EventHandler) Start(c *gin.Context) {
	var req StartRequest

	// Проверяем, что в запросе есть поле "type" и оно не пустое
	// Если нет — автоматически вернём 400 Bad Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Поле 'type' обязательно и не может быть пустым"})
		return
	}

	// Валидируем формат типа события согласно OpenAPI контракту: ^[a-z0-9]+$
	if !validateEventType(req.Type) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Тип события должен содержать только строчные буквы и цифры"})
		return
	}

	// Просим сервис запустить событие
	// Сервис сам решит, создавать ли новое или вернуть существующее
	event, err := h.service.Start(c.Request.Context(), req.Type)
	if err != nil {
		// Если что-то пошло не так — возвращаем ошибку 500
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Не удалось создать событие"})
		return
	}

	// Всё хорошо — возвращаем событие со статусом 200
	c.JSON(http.StatusOK, event.ToResponse())
}

// Finish обрабатывает запрос на завершение события
// Принимает JSON с полем "type" и завершает активное событие этого типа
func (h *EventHandler) Finish(c *gin.Context) {
	var req StartRequest

	// Проверяем, что в запросе есть поле "type"
	// Если нет — возвращаем 400 Bad Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Поле 'type' обязательно и не может быть пустым"})
		return
	}

	// Валидируем формат типа события согласно OpenAPI контракту: ^[a-z0-9]+$
	if !validateEventType(req.Type) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Тип события должен содержать только строчные буквы и цифры"})
		return
	}

	// Просим сервис завершить событие
	event, err := h.service.Finish(c.Request.Context(), req.Type)
	if err == mongo.ErrNoDocuments {
		// Если активного события такого типа нет — возвращаем 404 Not Found
		c.JSON(http.StatusNotFound, ErrorResponse{Message: "Активное событие указанного типа не найдено"})
		return
	}
	if err != nil {
		// Если произошла другая ошибка — возвращаем 500
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Не удалось завершить событие"})
		return
	}

	// Всё хорошо — возвращаем завершённое событие со статусом 200
	c.JSON(http.StatusOK, event.ToResponse())
}

// List обрабатывает запрос на получение списка всех событий
// Поддерживает query параметры: offset, limit, type
// Возвращает события, отсортированные по времени начала в порядке убывания (descending)
func (h *EventHandler) List(c *gin.Context) {
	// Парсим query параметры
	var offset int
	var limit int
	var eventType string

	if offsetStr := c.Query("offset"); offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Параметр 'offset' должен быть неотрицательным числом"})
			return
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 0 || limit > 100 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Параметр 'limit' должен быть числом от 0 до 100"})
			return
		}
	}

	eventType = c.Query("type")

	// Просим сервис вернуть события с учетом фильтров
	events, err := h.service.List(c.Request.Context(), offset, limit, eventType)
	if err != nil {
		// Если произошла ошибка — возвращаем 500
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Не удалось получить список событий"})
		return
	}

	// Всё хорошо — возвращаем список событий со статусом 200
	c.JSON(http.StatusOK, events)
}
