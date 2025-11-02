package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"event-service/internal/db"
	"event-service/pkg/event"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	baseURL = "http://localhost:8080/v1"
	port    = ":8080"
)

// TestResult представляет результат выполнения одного теста
type TestResult struct {
	Number   int
	Name     string
	Status   string // "PASS" или "FAIL"
	Message  string
	Expected string
	Actual   string
	Duration time.Duration
}

// TestSuite хранит все результаты тестов
type TestSuite struct {
	Results    []TestResult
	StartTime  time.Time
	EndTime    time.Time
	Passed     int
	Failed     int
	OutputFile *os.File
}

func NewTestSuite(outputFile string) (*TestSuite, error) {
	var file *os.File
	var err error

	if outputFile != "" {
		file, err = os.Create(outputFile)
		if err != nil {
			return nil, fmt.Errorf("не удалось создать файл результатов: %w", err)
		}
	}

	return &TestSuite{
		Results:    make([]TestResult, 0),
		OutputFile: file,
	}, nil
}

func (ts *TestSuite) Close() {
	if ts.OutputFile != nil {
		ts.OutputFile.Close()
	}
}

func (ts *TestSuite) Write(text string) {
	fmt.Print(text)
	if ts.OutputFile != nil {
		ts.OutputFile.WriteString(text)
	}
}

func (ts *TestSuite) Writef(format string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	ts.Write(text)
}

func (ts *TestSuite) AddResult(result TestResult) {
	ts.Results = append(ts.Results, result)
	if result.Status == "PASS" {
		ts.Passed++
	} else {
		ts.Failed++
	}
}

func (ts *TestSuite) PrintSummary() {
	ts.Write("\n" + strings.Repeat("=", 80) + "\n")
	ts.Writef("ИТОГОВЫЙ ОТЧЁТ\n")
	ts.Write(strings.Repeat("=", 80) + "\n")
	ts.Writef("Всего тестов: %d\n", len(ts.Results))
	ts.Writef("Пройдено:     %d\n", ts.Passed)
	ts.Writef("Провалено:   %d\n", ts.Failed)
	ts.Writef("Время выполнения: %v\n", ts.EndTime.Sub(ts.StartTime))
	ts.Write(strings.Repeat("=", 80) + "\n")

	if ts.Failed == 0 {
		ts.Write("\nВСЕ ТЕСТЫ ПРОЙДЕНЫ УСПЕШНО!\n")
	} else {
		ts.Write("\nЕСТЬ ПРОВАЛЕННЫЕ ТЕСТЫ\n")
	}
}

// EventResponse представляет событие из API согласно OpenAPI спецификации
type EventResponse struct {
	ID         string     `json:"id"` // ObjectID как строка
	Type       string     `json:"type"`
	State      string     `json:"state"`                // "started" или "finished"
	StartedAt  time.Time  `json:"startedAt"`            // camelCase
	FinishedAt *time.Time `json:"finishedAt,omitempty"` // camelCase
}

// isValidObjectID проверяет, что ID является валидным ObjectID MongoDB
// ObjectID должен быть строкой из 24 hex символов и не равняться дефолтному значению
func isValidObjectID(id string) bool {
	// Проверяем длину (ObjectID = 24 hex символа)
	if len(id) != 24 {
		return false
	}

	// Проверяем, что это не дефолтное пустое значение
	if id == "000000000000000000000000" {
		return false
	}

	// Проверяем, что все символы - hex (0-9, a-f, A-F)
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}

	return true
}

// ErrorResponse представляет ошибку из API согласно OpenAPI спецификации
type ErrorResponse struct {
	Message string `json:"message"`
}

func makeRequest(method, url string, body interface{}) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return resp, respBody, nil
}

func runTest(ts *TestSuite, num int, name string, testFunc func() error) {
	ts.Writef("\n[Test %d] %s\n", num, name)
	ts.Write(strings.Repeat("-", 60) + "\n")

	start := time.Now()
	err := testFunc()
	duration := time.Since(start)

	result := TestResult{
		Number:   num,
		Name:     name,
		Duration: duration,
	}

	if err != nil {
		result.Status = "FAIL"
		result.Message = err.Error()
		ts.Writef("FAIL: %s\n", err.Error())
	} else {
		result.Status = "PASS"
		result.Message = "Тест пройден"
		ts.Writef("PASS\n")
	}

	ts.Writef("Время выполнения: %v\n", duration)
	ts.AddResult(result)
}

// Test 1: GET empty array
func test1_GET_empty_array(ts *TestSuite) error {
	resp, body, err := makeRequest("GET", baseURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var events []EventResponse
	if err := json.Unmarshal(body, &events); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if len(events) != 0 {
		return fmt.Errorf("ожидался пустой массив, получено %d событий", len(events))
	}

	return nil
}

// Test 2: POST start meeting
func test2_POST_start_meeting(ts *TestSuite) error {
	resp, body, err := makeRequest("POST", baseURL+"/start", map[string]string{"type": "meeting"})
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d. Тело ответа: %s", resp.StatusCode, string(body))
	}

	// POST /v1/start возвращает событие с id в теле ответа
	var event EventResponse
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("ошибка парсинга JSON ответа: %v. Тело: %s", err, string(body))
	}

	// Проверяем наличие и валидность ID
	if event.ID == "" {
		return fmt.Errorf("ID события отсутствует в ответе")
	}

	if !isValidObjectID(event.ID) {
		return fmt.Errorf("невалидный ObjectID: '%s'. Ожидался валидный 24-символьный hex ID", event.ID)
	}

	if event.Type != "meeting" {
		return fmt.Errorf("ожидался тип 'meeting', получен '%s'", event.Type)
	}

	if event.State != "started" {
		return fmt.Errorf("ожидался state='started', получен state='%s'", event.State)
	}

	if event.FinishedAt != nil {
		return fmt.Errorf("ожидался finishedAt=nil для активного события")
	}

	ts.Writef("  Событие создано: id=%s, type=%s, state=%s\n", event.ID, event.Type, event.State)
	return nil
}

// Test 3: GET verify meeting
func test3_GET_verify_meeting(ts *TestSuite) error {
	resp, body, err := makeRequest("GET", baseURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var events []EventResponse
	if err := json.Unmarshal(body, &events); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if len(events) != 1 {
		return fmt.Errorf("ожидалось 1 событие, получено %d", len(events))
	}

	event := events[0]

	// Проверяем наличие и валидность ID
	if event.ID == "" {
		return fmt.Errorf("ID события отсутствует в ответе")
	}

	if !isValidObjectID(event.ID) {
		return fmt.Errorf("невалидный ObjectID: '%s'. Ожидался валидный 24-символьный hex ID", event.ID)
	}

	if event.Type != "meeting" {
		return fmt.Errorf("ожидался тип 'meeting', получен '%s'", event.Type)
	}

	if event.State != "started" {
		return fmt.Errorf("ожидался state='started', получен state='%s'", event.State)
	}

	if event.FinishedAt != nil {
		return fmt.Errorf("ожидался finishedAt=nil для активного события")
	}

	ts.Writef("  Событие: id=%s, type=%s, state=%s, startedAt=%v\n", event.ID, event.Type, event.State, event.StartedAt)
	return nil
}

// Test 4: POST duplicate meeting
func test4_POST_duplicate_meeting(ts *TestSuite) error {
	resp, body, err := makeRequest("POST", baseURL+"/start", map[string]string{"type": "meeting"})
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200 (не 400 и не 409!), получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	// POST /v1/start возвращает существующее событие с id в теле ответа
	var event EventResponse
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("ошибка парсинга JSON ответа: %v. Тело: %s", err, string(body))
	}

	// Проверяем наличие и валидность ID
	if event.ID == "" {
		return fmt.Errorf("ID события отсутствует в ответе")
	}

	if !isValidObjectID(event.ID) {
		return fmt.Errorf("невалидный ObjectID в ответе: '%s'", event.ID)
	}

	if event.Type != "meeting" {
		return fmt.Errorf("ожидался тип 'meeting', получен '%s'", event.Type)
	}

	if event.State != "started" {
		return fmt.Errorf("ожидался state='started', получен state='%s'", event.State)
	}

	ts.Writef("  Возвращено существующее событие: id=%s\n", event.ID)
	return nil
}

// Test 5: GET verify no duplicate
func test5_GET_verify_no_duplicate(ts *TestSuite) error {
	resp, body, err := makeRequest("GET", baseURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var events []EventResponse
	if err := json.Unmarshal(body, &events); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	meetingCount := 0
	for _, event := range events {
		// Проверяем валидность ID каждого события
		if event.ID == "" {
			return fmt.Errorf("найдено событие без ID")
		}

		if !isValidObjectID(event.ID) {
			return fmt.Errorf("найдено событие с невалидным ObjectID: '%s'", event.ID)
		}

		if event.Type == "meeting" && event.State == "started" {
			meetingCount++
		}
	}

	if meetingCount != 1 {
		return fmt.Errorf("ожидалось 1 активное событие 'meeting', найдено %d", meetingCount)
	}

	ts.Writef("  Всего событий: %d, активных 'meeting': %d (все ID валидны)\n", len(events), meetingCount)
	return nil
}

// Test 6: POST finish meeting
func test6_POST_finish_meeting(ts *TestSuite) error {
	resp, body, err := makeRequest("POST", baseURL+"/finish", map[string]string{"type": "meeting"})
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	// POST /v1/finish возвращает завершённое событие с id в теле ответа
	var event EventResponse
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("ошибка парсинга JSON ответа: %v. Тело: %s", err, string(body))
	}

	// Проверяем наличие и валидность ID
	if event.ID == "" {
		return fmt.Errorf("ID события отсутствует в ответе")
	}

	if !isValidObjectID(event.ID) {
		return fmt.Errorf("невалидный ObjectID в ответе: '%s'", event.ID)
	}

	if event.State != "finished" {
		return fmt.Errorf("ожидался state='finished', получен state='%s'", event.State)
	}

	if event.FinishedAt == nil {
		return fmt.Errorf("ожидался заполненный finishedAt для завершённого события")
	}

	ts.Writef("  Событие завершено: id=%s, finishedAt=%v\n", event.ID, event.FinishedAt)
	return nil
}

// Test 7: POST finish non-existent
func test7_POST_finish_nonexistent(ts *TestSuite) error {
	resp, body, err := makeRequest("POST", baseURL+"/finish", map[string]string{"type": "workout"})
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("ожидался статус 404, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	// Согласно OpenAPI спецификации, ошибка возвращается в формате Error
	var errorResp ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return fmt.Errorf("ошибка парсинга JSON ответа об ошибке: %v", err)
	}

	if errorResp.Message == "" {
		return fmt.Errorf("ожидалось сообщение об ошибке в поле 'message'")
	}

	ts.Writef("  Получена ожидаемая ошибка 404: %s\n", errorResp.Message)
	return nil
}

// Test 8 & 9: POST start call / task
func test8_POST_start_call(ts *TestSuite) error {
	resp, body, err := makeRequest("POST", baseURL+"/start", map[string]string{"type": "call"})
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	// POST /v1/start возвращает событие с id в теле ответа
	var event EventResponse
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("ошибка парсинга JSON ответа: %v. Тело: %s", err, string(body))
	}

	// Проверяем наличие и валидность ID
	if event.ID == "" {
		return fmt.Errorf("ID события отсутствует в ответе")
	}

	if !isValidObjectID(event.ID) {
		return fmt.Errorf("невалидный ObjectID: '%s'. Ожидался валидный 24-символьный hex ID", event.ID)
	}

	if event.Type != "call" {
		return fmt.Errorf("ожидался тип 'call', получен '%s'", event.Type)
	}

	if event.State != "started" {
		return fmt.Errorf("ожидался state='started', получен state='%s'", event.State)
	}

	ts.Writef("  Событие 'call' создано: id=%s\n", event.ID)
	return nil
}

func test9_POST_start_task(ts *TestSuite) error {
	resp, body, err := makeRequest("POST", baseURL+"/start", map[string]string{"type": "task"})
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	// POST /v1/start возвращает событие с id в теле ответа
	var event EventResponse
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("ошибка парсинга JSON ответа: %v. Тело: %s", err, string(body))
	}

	// Проверяем наличие и валидность ID
	if event.ID == "" {
		return fmt.Errorf("ID события отсутствует в ответе")
	}

	if !isValidObjectID(event.ID) {
		return fmt.Errorf("невалидный ObjectID: '%s'. Ожидался валидный 24-символьный hex ID", event.ID)
	}

	if event.Type != "task" {
		return fmt.Errorf("ожидался тип 'task', получен '%s'", event.Type)
	}

	if event.State != "started" {
		return fmt.Errorf("ожидался state='started', получен state='%s'", event.State)
	}

	ts.Writef("  Событие 'task' создано: id=%s\n", event.ID)
	return nil
}

// Test 10: GET verify both events
func test10_GET_verify_both_events(ts *TestSuite) error {
	resp, body, err := makeRequest("GET", baseURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var events []EventResponse
	if err := json.Unmarshal(body, &events); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if len(events) != 3 {
		return fmt.Errorf("ожидалось 3 события, получено %d", len(events))
	}

	// Проверяем наличие всех трёх событий
	hasMeeting := false
	hasCall := false
	hasTask := false

	for _, event := range events {
		switch event.Type {
		case "meeting":
			if event.State == "finished" {
				hasMeeting = true
			}
		case "call":
			if event.State == "started" {
				hasCall = true
			}
		case "task":
			if event.State == "started" {
				hasTask = true
			}
		}
	}

	if !hasMeeting {
		return fmt.Errorf("не найдено завершённое событие 'meeting'")
	}
	if !hasCall {
		return fmt.Errorf("не найдено активное событие 'call'")
	}
	if !hasTask {
		return fmt.Errorf("не найдено активное событие 'task'")
	}

	// Проверяем валидность ID всех событий
	for i, event := range events {
		if event.ID == "" {
			return fmt.Errorf("событие %d не имеет ID", i+1)
		}

		if !isValidObjectID(event.ID) {
			return fmt.Errorf("событие %d имеет невалидный ObjectID: '%s'", i+1, event.ID)
		}
	}

	// Проверяем сортировку по startedAt в порядке убывания (descending)
	// Согласно OpenAPI спецификации: "Returns a list of events sorted by start time in descending order"
	for i := 1; i < len(events); i++ {
		if events[i].StartedAt.After(events[i-1].StartedAt) {
			return fmt.Errorf("события не отсортированы по startedAt в порядке убывания: событие %d началось позже события %d", i, i-1)
		}
	}

	ts.Writef("  Все события найдены, отсортированы и имеют валидные ID:\n")
	for i, event := range events {
		finishedAtStr := "nil"
		if event.FinishedAt != nil {
			finishedAtStr = event.FinishedAt.Format(time.RFC3339)
		}
		ts.Writef("    %d. id=%s, type=%s, state=%s, startedAt=%v, finishedAt=%s\n", i+1, event.ID, event.Type, event.State, event.StartedAt.Format(time.RFC3339), finishedAtStr)
	}

	return nil
}

func setupTestServer() (*mongo.Client, func(), error) {
	// Запускаем встроенный MongoDB
	mongoURI, cleanupMongo, err := db.StartEmbeddedMongo()
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось запустить MongoDB: %w", err)
	}

	// Подключаемся к MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		cleanupMongo()
		return nil, nil, fmt.Errorf("не удалось подключиться к MongoDB: %w", err)
	}

	// Проверяем подключение
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		cleanupMongo()
		return nil, nil, fmt.Errorf("не удалось проверить соединение: %w", err)
	}

	// Создаём коллекцию (используем уникальную базу для тестов)
	collection := client.Database("events_test_db").Collection("events")

	// Очищаем коллекцию перед тестами
	collection.Drop(ctx)

	// Создаём репозиторий, сервис и обработчик
	repo := event.NewEventRepository(collection)
	service := event.NewEventService(repo)
	handler := event.NewEventHandler(service)

	// Настраиваем Gin
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	v1 := r.Group("/v1")
	{
		v1.GET("", handler.List)
		v1.POST("/start", handler.Start)
		v1.POST("/finish", handler.Finish)
	}

	// Запускаем сервер в горутине
	go func() {
		if err := r.Run(port); err != nil && err != http.ErrServerClosed {
			log.Printf("Ошибка запуска сервера: %v", err)
		}
	}()

	// Ждём, пока сервер запустится
	time.Sleep(1 * time.Second)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Disconnect(ctx)
		cleanupMongo()
	}

	return client, cleanup, nil
}

func main() {
	fmt.Println("Запуск интеграционных тестов для event-service")
	fmt.Println(strings.Repeat("=", 80))

	// Определяем файл для результатов (опционально через аргумент)
	outputFile := ""
	if len(os.Args) > 1 {
		outputFile = os.Args[1]
		fmt.Printf("Результаты будут сохранены в файл: %s\n", outputFile)
	}

	// Создаём набор тестов
	ts, err := NewTestSuite(outputFile)
	if err != nil {
		log.Fatalf("Ошибка создания набора тестов: %v", err)
	}
	defer ts.Close()

	ts.StartTime = time.Now()

	// Настраиваем тестовый сервер
	ts.Write("\nНастройка тестового окружения...\n")
	client, cleanup, err := setupTestServer()
	if err != nil {
		log.Fatalf("Ошибка настройки тестового сервера: %v", err)
	}
	defer cleanup()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Очищаем тестовую базу
		client.Database("events_test_db").Collection("events").Drop(ctx)
	}()

	ts.Write("Тестовый сервер запущен\n\n")

	// Запускаем все тесты
	runTest(ts, 1, "GET empty array", func() error {
		return test1_GET_empty_array(ts)
	})

	runTest(ts, 2, "POST start meeting", func() error {
		return test2_POST_start_meeting(ts)
	})

	runTest(ts, 3, "GET verify meeting", func() error {
		return test3_GET_verify_meeting(ts)
	})

	runTest(ts, 4, "POST duplicate meeting (should return 200, not create duplicate)", func() error {
		return test4_POST_duplicate_meeting(ts)
	})

	runTest(ts, 5, "GET verify no duplicate", func() error {
		return test5_GET_verify_no_duplicate(ts)
	})

	runTest(ts, 6, "POST finish meeting", func() error {
		return test6_POST_finish_meeting(ts)
	})

	runTest(ts, 7, "POST finish non-existent (should return 404)", func() error {
		return test7_POST_finish_nonexistent(ts)
	})

	runTest(ts, 8, "POST start call", func() error {
		return test8_POST_start_call(ts)
	})

	runTest(ts, 9, "POST start task", func() error {
		return test9_POST_start_task(ts)
	})

	runTest(ts, 10, "GET verify both events (call, task) and sorting", func() error {
		return test10_GET_verify_both_events(ts)
	})

	ts.EndTime = time.Now()

	// Выводим итоговый отчёт
	ts.PrintSummary()

	// Завершаем с соответствующим кодом
	if ts.Failed > 0 {
		os.Exit(1)
	}
}
