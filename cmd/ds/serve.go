package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
)

func serveCommand(ctx *cli.Context) error {
	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	// Настройки сервера
	host := ctx.String("host")
	port := ctx.Int("port")
	unixSocket := ctx.String("unix-socket")
	enableCORS := ctx.Bool("cors")
	logRequests := ctx.Bool("log-requests")

	// Создаем API сервер
	apiServer := &APIServer{
		app:         app,
		enableCORS:  enableCORS,
		logRequests: logRequests,
	}

	// Настраиваем маршруты
	router := mux.NewRouter()
	apiServer.setupRoutes(router)

	server := &http.Server{
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Канал для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем сервер
	var listener net.Listener
	if unixSocket != "" {
		// Unix socket
		os.Remove(unixSocket) // Удаляем старый сокет если есть
		listener, err = net.Listen("unix", unixSocket)
		if err != nil {
			return fmt.Errorf("ошибка создания Unix socket: %w", err)
		}

		// Устанавливаем права доступа
		if err := os.Chmod(unixSocket, 0666); err != nil {
			return fmt.Errorf("ошибка установки прав доступа для сокета: %w", err)
		}

		fmt.Printf("🚀 Сервер запущен на Unix socket: %s\n", unixSocket)
	} else {
		// TCP
		addr := fmt.Sprintf("%s:%d", host, port)
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("ошибка создания TCP сервера: %w", err)
		}

		fmt.Printf("🚀 Сервер запущен на http://%s\n", addr)
	}

	fmt.Printf("📚 API документация доступна по адресу /api/docs\n")
	fmt.Printf("📊 Статистика доступна по адресу /api/stats\n")
	fmt.Printf("❌ Для остановки нажмите Ctrl+C\n\n")

	// Запускаем сервер в горутине
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Ошибка сервера: %v", err)
		}
	}()

	// Ждем сигнал для завершения
	<-quit
	fmt.Println("\n🛑 Получен сигнал завершения...")

	// Graceful shutdown
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		fmt.Printf("⚠️  Ошибка при graceful shutdown: %v\n", err)
		return err
	}

	// Удаляем Unix socket при завершении
	if unixSocket != "" {
		os.Remove(unixSocket)
	}

	fmt.Println("✅ Сервер корректно остановлен")
	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "serve",
		Aliases: []string{"server", "srv"},
		Usage:   "Запустить веб-сервер с REST API",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "host",
				Aliases: []string{"H"},
				Value:   "localhost",
				Usage:   "Хост для привязки сервера",
				EnvVars: []string{"UES_SERVER_HOST"},
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8080,
				Usage:   "Порт для сервера",
				EnvVars: []string{"UES_SERVER_PORT"},
			},
			&cli.StringFlag{
				Name:    "unix-socket",
				Aliases: []string{"u"},
				Usage:   "Путь к Unix socket (альтернатива TCP)",
				EnvVars: []string{"UES_UNIX_SOCKET"},
			},
			&cli.BoolFlag{
				Name:    "cors",
				Aliases: []string{"c"},
				Value:   true,
				Usage:   "Включить CORS поддержку",
			},
			&cli.BoolFlag{
				Name:    "log-requests",
				Aliases: []string{"l"},
				Value:   true,
				Usage:   "Логировать HTTP запросы",
			},
		},
		Action: serveCommand,
		Description: `Запускает веб-сервер с REST API для работы с датастором.

Сервер может работать через TCP или Unix socket.

REST API эндпоинты:
  GET    /api/keys                 - список ключей
  GET    /api/keys/{key}           - получить значение ключа
  PUT    /api/keys/{key}           - установить значение ключа
  DELETE /api/keys/{key}           - удалить ключ
  GET    /api/keys/{key}/info      - информация о ключе
  POST   /api/search               - поиск ключей
  GET    /api/stats                - статистика датастора
  DELETE /api/clear                - очистить все ключи
  POST   /api/export               - экспорт данных
  POST   /api/import               - импорт данных
  GET    /api/subscriptions        - список подписок
  POST   /api/subscriptions        - создать подписку
  DELETE /api/subscriptions/{id}   - удалить подписку

Примеры:
  ues-ds serve                                    # TCP на localhost:8080
  ues-ds serve --port 3000 --host 0.0.0.0       # Публичный доступ
  ues-ds serve --unix-socket /tmp/ues-ds.sock    # Unix socket
  ues-ds serve --cors=false --log-requests=false # Без CORS и логов`,
	})
}
