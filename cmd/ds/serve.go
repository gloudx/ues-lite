package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"ues-lite/datastore"

	"github.com/urfave/cli/v2"
)

func serveCommand(ctx *cli.Context) error {
	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	// Создаем конфигурацию сервера
	config := &datastore.Config{
		Host:                 ctx.String("host"),
		Port:                 ctx.Int("port"),
		EnableCORS:           ctx.Bool("cors"),
		EnableMetrics:        ctx.Bool("metrics"),
		EnableAuth:           ctx.Bool("auth"),
		AuthToken:            ctx.String("auth-token"),
		LogRequests:          ctx.Bool("log-requests"),
		RequestTimeout:       ctx.Duration("request-timeout"),
		ReadTimeout:          ctx.Duration("read-timeout"),
		WriteTimeout:         ctx.Duration("write-timeout"),
		IdleTimeout:          ctx.Duration("idle-timeout"),
		ShutdownTimeout:      ctx.Duration("shutdown-timeout"),
		MaxRequestSize:       ctx.Int64("max-request-size"),
		RateLimitRPS:         ctx.Float64("rate-limit-rps"),
		RateLimitBurst:       ctx.Int("rate-limit-burst"),
		EnableCompression:    ctx.Bool("compression"),
		EnableStructuredLogs: ctx.Bool("structured-logs"),
	}

	// Создаем API сервер
	server := datastore.NewAPIServer(app.ds, config)

	// Создаем контекст для graceful shutdown
	serverCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем сервер в горутине
	errChan := make(chan error, 1)
	go func() {
		fmt.Printf("🚀 Запуск API сервера на %s:%d\n", config.Host, config.Port)

		if config.EnableMetrics {
			fmt.Printf("📊 Метрики доступны на http://%s:%d/metrics\n", config.Host, config.Port)
		}

		fmt.Printf("📚 API документация: http://%s:%d/api/v1/docs\n", config.Host, config.Port)

		if err := server.Start(serverCtx); err != nil {
			errChan <- err
		}
	}()

	// Ожидаем сигнал завершения или ошибку
	select {
	case <-sigChan:
		fmt.Println("\n🛑 Получен сигнал завершения, останавливаем сервер...")
		cancel()
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("ошибка запуска сервера: %w", err)
		}
	}

	fmt.Println("✅ Сервер остановлен")
	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "serve",
		Aliases: []string{"server", "api"},
		Usage:   "Запустить HTTP API сервер",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "host",
				Aliases: []string{"H"},
				Value:   "localhost",
				Usage:   "Хост для привязки сервера",
				EnvVars: []string{"UES_HOST"},
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8080,
				Usage:   "Порт для сервера",
				EnvVars: []string{"UES_PORT"},
			},
			&cli.BoolFlag{
				Name:  "cors",
				Value: true,
				Usage: "Включить поддержку CORS",
			},
			&cli.BoolFlag{
				Name:  "metrics",
				Value: true,
				Usage: "Включить Prometheus метрики",
			},
			&cli.BoolFlag{
				Name:  "auth",
				Usage: "Включить аутентификацию",
			},
			&cli.StringFlag{
				Name:    "auth-token",
				Usage:   "Токен для аутентификации (если включена)",
				EnvVars: []string{"UES_AUTH_TOKEN"},
			},
			&cli.BoolFlag{
				Name:  "log-requests",
				Value: true,
				Usage: "Логировать HTTP запросы",
			},
			&cli.DurationFlag{
				Name:  "request-timeout",
				Value: 30000000000, // 30s
				Usage: "Таймаут обработки запроса",
			},
			&cli.DurationFlag{
				Name:  "read-timeout",
				Value: 30000000000, // 30s
				Usage: "Таймаут чтения запроса",
			},
			&cli.DurationFlag{
				Name:  "write-timeout",
				Value: 30000000000, // 30s
				Usage: "Таймаут записи ответа",
			},
			&cli.DurationFlag{
				Name:  "idle-timeout",
				Value: 60000000000, // 60s
				Usage: "Таймаут простоя соединения",
			},
			&cli.DurationFlag{
				Name:  "shutdown-timeout",
				Value: 30000000000, // 30s
				Usage: "Таймаут graceful shutdown",
			},
			&cli.Int64Flag{
				Name:  "max-request-size",
				Value: 33554432, // 32MB
				Usage: "Максимальный размер запроса в байтах",
			},
			&cli.Float64Flag{
				Name:  "rate-limit-rps",
				Value: 100.0,
				Usage: "Лимит запросов в секунду",
			},
			&cli.IntFlag{
				Name:  "rate-limit-burst",
				Value: 200,
				Usage: "Размер буфера для rate limiting",
			},
			&cli.BoolFlag{
				Name:  "compression",
				Value: true,
				Usage: "Включить сжатие ответов",
			},
			&cli.BoolFlag{
				Name:  "structured-logs",
				Value: true,
				Usage: "Структурированное логирование",
			},
		},
		Action: serveCommand,
		Description: `Запускает HTTP API сервер для датастора.

Сервер предоставляет RESTful API для работы с датастором, включая:
- CRUD операции с ключами
- JQ запросы и агрегации
- Views (представления данных)
- Transform операции
- Streaming данных
- JavaScript подписки на события
- Метрики Prometheus

Примеры:
  # Запуск на порту 8080
  ues-ds serve
  
  # Запуск на другом порту с аутентификацией
  ues-ds serve --port 9000 --auth --auth-token secret123
  
  # Запуск с настроенными таймаутами
  ues-ds serve --request-timeout 60s --rate-limit-rps 200

Переменные окружения:
  UES_HOST         - хост сервера
  UES_PORT         - порт сервера  
  UES_AUTH_TOKEN   - токен аутентификации`,
	})
}
