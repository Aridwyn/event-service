# event-service

Сервис для управления событиями на Go с использованием Gin и MongoDB.

## Особенности

- **Кроссплатформенный запуск без установки MongoDB** — встроенный MongoDB запускается автоматически
- Работает на **Windows, Linux, macOS**
- Автоматическая очистка временных файлов при завершении работы

## Быстрый старт

### Со встроенным MongoDB (по умолчанию)

Просто запустите приложение:

```bash
go run ./cmd/event-service
```

MongoDB запустится автоматически из встроенного бинарника. База данных будет храниться во временной папке, которая автоматически удалится при завершении работы приложения.

### С внешним MongoDB

Если у вас уже установлен MongoDB и вы хотите использовать его:

```bash
# Windows PowerShell
$env:MONGO_URI="mongodb://localhost:27017/events_db"
go run ./cmd/event-service

# Linux/macOS
export MONGO_URI="mongodb://localhost:27017/events_db"
go run ./cmd/event-service
```

## API Эндпоинты

- `GET /v1` — получить список всех событий, отсортированных по времени начала (по возрастанию)
- `POST /v1/start` — создать новое событие указанного типа. Если активное событие этого типа уже есть — ничего не делает, возвращает существующее (200 OK)
- `POST /v1/finish` — завершить активное событие указанного типа. Если такого события нет — возвращает 404 Not Found

### Примеры использования

**Создать событие:**
```bash
# Windows PowerShell
Invoke-RestMethod -Uri "http://localhost:8080/v1/start" -Method POST -Body '{"type":"login"}' -ContentType "application/json"

# Linux/macOS
curl -X POST http://localhost:8080/v1/start -H "Content-Type: application/json" -d '{"type":"login"}'
```

**Получить все события:**
```bash
# Windows PowerShell
Invoke-RestMethod -Uri "http://localhost:8080/v1" -Method GET

# Linux/macOS
curl http://localhost:8080/v1
```

**Завершить событие:**
```bash
# Windows PowerShell
Invoke-RestMethod -Uri "http://localhost:8080/v1/finish" -Method POST -Body '{"type":"login"}' -ContentType "application/json"

# Linux/macOS
curl -X POST http://localhost:8080/v1/finish -H "Content-Type: application/json" -d '{"type":"login"}'
```

## Структура проекта

```
event-service/
├── cmd/event-service/
│   └── main.go              # Точка входа приложения
├── pkg/event/
│   ├── model.go             # Модель события
│   ├── repository.go        # Работа с MongoDB
│   ├── service.go           # Бизнес-логика
│   └── handler.go           # HTTP-обработчики
├── embedded/
│   ├── mongod.go            # Встраивание бинарников MongoDB
│   ├── mongod-windows-amd64.exe
│   ├── mongod-linux-amd64
│   └── mongod-darwin-amd64
└── internal/db/
    └── embedded_mongo.go    # Логика запуска встроенного MongoDB
```

## Важные замечания

- **Бинарники MongoDB** взяты из официальных архивов MongoDB (лицензия SSPL)
- **Для production** рекомендуется использовать внешний MongoDB
- Встроенный MongoDB запускается с параметром `--nojournal` для быстрого старта в dev окружении
- Все данные встроенного MongoDB хранятся во временной папке и удаляются при завершении работы

## Требования

- Go 1.23 или выше
