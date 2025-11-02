# Linux бинарники для тестирования

Эта папка содержит скомпилированные Linux-бинарники для тестирования event-service.

## Содержимое

- `test-runner` - Интеграционные тесты (основной инструмент для тестирования)
- `event-service` - Основной сервис (для ручного тестирования)

## Интеграционные тесты

### Быстрый запуск

```bash
# Просто запустить все интеграционные тесты
./test-runner
```

### Сохранение результатов в файл

```bash
# Результаты будут сохранены в файл test-results.txt
./test-runner test-results.txt
```

### Что делают интеграционные тесты

Тесты автоматически:
1. Запускают встроенный MongoDB для тестов
2. Запускают тестовый HTTP-сервер на порту 8080
3. Выполняют 10 интеграционных тест-кейсов:
   - GET empty array - проверка пустой базы
   - POST start meeting - создание события типа "meeting"
   - GET verify meeting - проверка сохранения события
   - POST duplicate meeting - проверка отсутствия дубликатов
   - GET verify no duplicate - подтверждение отсутствия дубликата
   - POST finish meeting - завершение события
   - POST finish non-existent - обработка ошибки 404
   - POST start call - создание события типа "call"
   - POST start task - создание события типа "task"
   - GET verify both events - проверка сортировки и независимости типов
4. Выводят результаты в консоль и (опционально) в файл
5. Автоматически очищают тестовую базу данных

### Пример вывода

```
Запуск интеграционных тестов для event-service
================================================================================

Настройка тестового окружения...
Тестовый сервер запущен

[Test 1] GET empty array
------------------------------------------------------------
PASS
Время выполнения: 45.2ms

[Test 2] POST start meeting
------------------------------------------------------------
  Событие создано: id=..., type=meeting, state=started
PASS
Время выполнения: 32.1ms

...

================================================================================
ИТОГОВЫЙ ОТЧЁТ
================================================================================
Всего тестов: 10
Пройдено:     10
Провалено:   0
Время выполнения: 2.3s
================================================================================

ВСЕ ТЕСТЫ ПРОЙДЕНЫ УСПЕШНО!
```

### Выходные коды

- `0` - все тесты пройдены успешно
- `1` - есть проваленные тесты

Это позволяет использовать в CI/CD:

```bash
./test-runner && echo "Tests passed" || echo "Tests failed"
```

## Юнит-тесты (через Go test)

Если вы хотите запустить юнит-тесты на Linux-машине, используйте стандартные команды Go:

```bash
# Все тесты из корня проекта
go test ./...

# Только тесты пакета event
go test ./pkg/event/...

# Только тесты main.go
go test ./cmd/event-service/

# С покрытием
go test -cover ./...

# С детальным выводом
go test -v ./...
```

## Ручное тестирование сервиса

Если нужно протестировать сервис вручную:

```bash
# Запуск сервиса
./event-service

# Сервис запустится на порту 8080
# Доступные эндпоинты:
#   GET  /v1 - получить список всех событий
#   POST /v1/start - создать новое событие
#   POST /v1/finish - завершить событие
```

### Примеры запросов

```bash
# Получить список событий
curl http://localhost:8080/v1

# Создать событие типа "meeting"
curl -X POST http://localhost:8080/v1/start \
  -H "Content-Type: application/json" \
  -d '{"type": "meeting"}'

# Завершить событие типа "meeting"
curl -X POST http://localhost:8080/v1/finish \
  -H "Content-Type: application/json" \
  -d '{"type": "meeting"}'
```

## Требования

- Linux (x86_64/amd64)
- Встроенный MongoDB будет запущен автоматически при использовании `test-runner`
- Для ручного запуска `event-service` можно использовать внешний MongoDB через переменную окружения `MONGO_URI`

## Пересборка бинарников

Если нужно пересобрать бинарники на Linux-машине:

```bash
# Из корня проекта
go build -o bin/linux/test-runner ./cmd/test-runner
go build -o bin/linux/event-service ./cmd/event-service
```

Или кросс-компиляция с Windows:

```powershell
# PowerShell
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o bin/linux/test-runner ./cmd/test-runner
go build -o bin/linux/event-service ./cmd/event-service
```

```bash
# Bash (на Linux/Mac или WSL)
GOOS=linux GOARCH=amd64 go build -o bin/linux/test-runner ./cmd/test-runner
GOOS=linux GOARCH=amd64 go build -o bin/linux/event-service ./cmd/event-service
```

