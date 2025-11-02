package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"event-service/embedded"
)

// StartEmbeddedMongo запускает встроенный MongoDB и возвращает URI для подключения
// Также возвращает функцию cleanup для корректного завершения процесса
// Временная папка с данными будет автоматически удалена при вызове cleanup
func StartEmbeddedMongo() (uri string, cleanup func(), err error) {
	// Создаём временную папку для хранения данных MongoDB
	// Эта папка будет удалена при завершении работы приложения
	tempDir, err := os.MkdirTemp("", "embedded-mongo-*")
	if err != nil {
		return "", nil, fmt.Errorf("не удалось создать временную папку: %w", err)
	}

	// Определяем, какой бинарник нужно использовать в зависимости от операционной системы
	var mongodData []byte
	var mongodName string

	goos := runtime.GOOS
	switch goos {
	case "windows":
		mongodData = embedded.MongodWindows
		mongodName = "mongod.exe"
	case "linux":
		mongodData = embedded.MongodLinux
		mongodName = "mongod"
	case "darwin":
		mongodData = embedded.MongodDarwin
		mongodName = "mongod"
	default:
		return "", nil, fmt.Errorf("неподдерживаемая платформа: %s", goos)
	}

	// Проверяем, что бинарник действительно есть
	// Если бинарники не были встроены, embedded пакет будет пустым
	if len(mongodData) == 0 {
		return "", nil, fmt.Errorf("бинарник MongoDB для %s не найден. Пожалуйста, скачайте его и поместите в папку embedded/", goos)
	}

	// Путь, куда сохраним извлечённый бинарник
	mongodPath := filepath.Join(tempDir, mongodName)

	// Сохраняем бинарник во временную папку
	err = os.WriteFile(mongodPath, mongodData, 0755)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("не удалось сохранить бинарник MongoDB: %w", err)
	}

	// На Linux и macOS нужно сделать файл исполняемым
	if goos != "windows" {
		err = os.Chmod(mongodPath, 0755)
		if err != nil {
			os.RemoveAll(tempDir)
			return "", nil, fmt.Errorf("не удалось сделать бинарник исполняемым: %w", err)
		}
	}

	// Создаём папку для данных MongoDB внутри временной директории
	dbPath := filepath.Join(tempDir, "data")
	err = os.MkdirAll(dbPath, 0755)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("не удалось создать папку для данных: %w", err)
	}

	// Запускаем MongoDB с нужными параметрами
	// --dbpath: где хранить данные
	// --port: на каком порту слушать (27017 — стандартный порт MongoDB)
	// --bind_ip: только локальные подключения (127.0.0.1)
	// --quiet: меньше логов
	// --nojournal: отключаем journal для быстрого старта (для dev окружения)
	cmd := exec.Command(mongodPath,
		"--dbpath", dbPath,
		"--port", "27017",
		"--bind_ip", "127.0.0.1",
		"--quiet",
		"--nojournal",
	)

	// Направляем вывод MongoDB в никуда, чтобы не засорять консоль
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Запускаем процесс MongoDB
	err = cmd.Start()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("не удалось запустить MongoDB: %w", err)
	}

	// Функция для корректного завершения работы MongoDB
	cleanupFunc := func() {
		log.Println("Останавливаем встроенный MongoDB...")

		// Завершаем процесс MongoDB
		// Используем Kill вместо Wait, чтобы точно убить процесс
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Process.Wait()
		}

		// Удаляем временную папку со всеми данными
		err := os.RemoveAll(tempDir)
		if err != nil {
			log.Printf("Не удалось удалить временную папку %s: %v", tempDir, err)
		} else {
			log.Println("Встроенный MongoDB остановлен, временные файлы удалены")
		}
	}

	// Ждём, пока MongoDB запустится и будет готов к работе
	// Даём ему до 10 секунд на запуск
	log.Println("Ожидание запуска встроенного MongoDB...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Простое ожидание — даём MongoDB время запуститься
	// В production можно было бы делать ping, но для простоты используем задержку
	select {
	case <-time.After(2 * time.Second):
		// MongoDB должен был запуститься за это время
	case <-ctx.Done():
		cleanupFunc()
		return "", nil, fmt.Errorf("таймаут ожидания запуска MongoDB")
	}

	log.Println("Встроенный MongoDB запущен на порту :27017")

	// Возвращаем URI для подключения и функцию очистки
	uri = "mongodb://localhost:27017/events_db"
	return uri, cleanupFunc, nil
}

// CheckMongoBinaryExists проверяет, есть ли встроенный бинарник для текущей платформы
// Полезно для диагностики перед запуском
func CheckMongoBinaryExists() bool {
	goos := runtime.GOOS
	switch goos {
	case "windows":
		return len(embedded.MongodWindows) > 0
	case "linux":
		return len(embedded.MongodLinux) > 0
	case "darwin":
		return len(embedded.MongodDarwin) > 0
	default:
		return false
	}
}
