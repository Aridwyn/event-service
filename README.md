# event-service

Сервис для управления событиями на Go с использованием Gin и MongoDB.

## Особенности

- **Кроссплатформенный запуск без установки MongoDB** — встроенный MongoDB запускается автоматически
- Работает на **Windows, Linux, macOS**
- Автоматическая очистка временных файлов при завершении работы

## Быстрый старт

### Со встроенным MongoDB (по умолчанию)

Для работы со встроенным MongoDB необходимо загрузить бинарники MongoDB и разместить их в папке `embedded/`.

#### Загрузка бинарников MongoDB

1. **Windows (amd64)**:
   - Скачайте MongoDB Community Server для Windows: [https://www.mongodb.com/try/download/community](https://www.mongodb.com/try/download/community?tck=docs_server)
   - Распакуйте архив и скопируйте файл `mongod.exe` из папки `bin/` в `embedded/` с именем `mongod-windows-amd64.exe`

2. **Linux (amd64)**:
   - Скачайте MongoDB Community Server для Linux: [https://www.mongodb.com/try/download/community](https://www.mongodb.com/try/download/community?tck=docs_server)
   - Распакуйте архив (`.tgz`) и скопируйте файл `mongod` из папки `bin/` в `embedded/` с именем `mongod-linux-amd64`
   - Убедитесь, что файл имеет права на выполнение: `chmod +x embedded/mongod-linux-amd64`

3. **macOS (darwin, amd64)**:
   - Скачайте MongoDB Community Server для macOS: [https://www.mongodb.com/try/download/community](https://www.mongodb.com/try/download/community?tck=docs_server)
   - Распакуйте архив (`.tgz`) и скопируйте файл `mongod` из папки `bin/` в `embedded/` с именем `mongod-darwin-amd64`
   - Убедитесь, что файл имеет права на выполнение: `chmod +x embedded/mongod-darwin-amd64`

**Прямые ссылки на официальные архивы MongoDB:**

Рекомендуемая версия для проекта: **MongoDB 6.0.x** (протестировано с версией 6.0.17).

Прямые ссылки на fastdl.mongodb.org (замените `X.X.X` на конкретную версию, например `6.0.17`):

**Windows (amd64, legacy):**
- `https://fastdl.mongodb.org/windows/mongodb-windows-x86_64-X.X.X.zip`
- Для Windows 7/Server 2008 R2 используйте legacy версию

**Linux (amd64):**
- Ubuntu 20.04: `https://fastdl.mongodb.org/linux/mongodb-linux-x86_64-ubuntu2004-X.X.X.tgz`
- Ubuntu 22.04: `https://fastdl.mongodb.org/linux/mongodb-linux-x86_64-ubuntu2204-X.X.X.tgz`
- RHEL 8: `https://fastdl.mongodb.org/linux/mongodb-linux-x86_64-rhel80-X.X.X.tgz`

**macOS (amd64):**
- `https://fastdl.mongodb.org/osx/mongodb-macos-x86_64-X.X.X.tgz`

Альтернативно, используйте официальную страницу загрузки:
- **Официальная страница**: [https://www.mongodb.com/try/download/community](https://www.mongodb.com/try/download/community)
- После выбора платформы и версии, скачайте архив и извлеките бинарник `mongod` (или `mongod.exe` для Windows) из папки `bin/`

После размещения бинарников запустите приложение:

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

- **Бинарники MongoDB** необходимо загрузить вручную и разместить в папке `embedded/` (см. раздел "Быстрый старт" выше)
- Бинарники берутся из официальных архивов MongoDB Community Server (лицензия SSPL)
- **Для production** рекомендуется использовать внешний MongoDB через переменную окружения `MONGO_URI`
- Встроенный MongoDB запускается с параметром `--nojournal` для быстрого старта в dev окружении
- Все данные встроенного MongoDB хранятся во временной папке и удаляются при завершении работы

## Требования

- Go 1.23 или выше
