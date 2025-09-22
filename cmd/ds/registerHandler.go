package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"ues-lite/datastore"

	"github.com/urfave/cli/v2"
)

func registerHandler(ctx *cli.Context) error {

	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ID обработчика")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	id := ctx.Args().Get(0)
	var script string

	// Получаем скрипт из аргумента или файла
	if ctx.NArg() >= 2 {
		script = ctx.Args().Get(1)
	} else if scriptFile := ctx.String("file"); scriptFile != "" {
		scriptBytes, err := os.ReadFile(scriptFile)
		if err != nil {
			return fmt.Errorf("ошибка чтения файла скрипта: %w", err)
		}
		script = string(scriptBytes)
	} else if ctx.Bool("stdin") {
		scriptBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("ошибка чтения скрипта из stdin: %w", err)
		}
		script = string(scriptBytes)
	} else {
		return fmt.Errorf("требуется скрипт: используйте аргумент, --file или --stdin")
	}

	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("скрипт не может быть пустым")
	}

	// Подготавливаем конфигурацию
	config := &datastore.JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: ctx.Duration("timeout"),
		EnableNetworking: ctx.Bool("networking"),
		EnableLogging:    ctx.Bool("logging"),
		StrictMode:       ctx.Bool("strict"),
	}

	// Парсим фильтры событий
	if eventFilters := ctx.StringSlice("events"); len(eventFilters) > 0 {
		var filters []datastore.EventType
		for _, filter := range eventFilters {
			switch strings.ToLower(filter) {
			case "put":
				filters = append(filters, datastore.EventPut)
			case "delete":
				filters = append(filters, datastore.EventDelete)
			case "batch":
				filters = append(filters, datastore.EventBatch)
			default:
				return fmt.Errorf("неизвестный тип события: %s (доступные: put, delete, batch)", filter)
			}
		}
		config.EventFilters = filters
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = app.ds.CreateJSSubscription(ctxTimeout, id, script, config)
	if err != nil {
		return fmt.Errorf("ошибка при регистрации обработчика: %w", err)
	}

	fmt.Printf("✅ Обработчик '%s' зарегистрирован\n", id)

	if len(config.EventFilters) > 0 {
		var eventNames []string
		for _, eventType := range config.EventFilters {
			switch eventType {
			case datastore.EventPut:
				eventNames = append(eventNames, "put")
			case datastore.EventDelete:
				eventNames = append(eventNames, "delete")
			case datastore.EventBatch:
				eventNames = append(eventNames, "batch")
			}
		}
		fmt.Printf("📋 Фильтры событий: %s\n", strings.Join(eventNames, ", "))
	} else {
		fmt.Printf("📋 Фильтры событий: все события\n")
	}

	fmt.Printf("⏱️  Таймаут выполнения: %v\n", config.ExecutionTimeout)
	fmt.Printf("🌐 Сетевой доступ: %t\n", config.EnableNetworking)
	fmt.Printf("📝 Логирование: %t\n", config.EnableLogging)

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "subscribe",
		Aliases: []string{"reg", "sub"},
		Usage:   "Зарегистрировать обработчик событий JavaScript",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "Путь к файлу со скриптом",
			},
			&cli.BoolFlag{
				Name:    "stdin",
				Aliases: []string{"s"},
				Usage:   "Читать скрипт из stdin",
			},
			&cli.DurationFlag{
				Name:    "timeout",
				Aliases: []string{"t"},
				Value:   5 * time.Second,
				Usage:   "Таймаут выполнения скрипта (например: 5s, 1m)",
			},
			&cli.BoolFlag{
				Name:    "networking",
				Aliases: []string{"n"},
				Value:   true,
				Usage:   "Включить сетевой доступ для скрипта",
			},
			&cli.BoolFlag{
				Name:    "logging",
				Aliases: []string{"l"},
				Value:   true,
				Usage:   "Включить логирование для скрипта",
			},
			&cli.BoolFlag{
				Name:  "strict",
				Usage: "Включить строгий режим JavaScript",
			},
			&cli.StringSliceFlag{
				Name:    "events",
				Aliases: []string{"e"},
				Usage:   "Фильтр типов событий (put, delete, batch). Можно указать несколько раз",
			},
		},
		Action:    registerHandler,
		ArgsUsage: "<ID> [скрипт]",
		Description: `Регистрирует обработчик событий JavaScript в датасторе.
Скрипт можно передать тремя способами:
1. Как второй аргумент: register my-handler "console.log('Hello')"
2. Из файла: register my-handler --file script.js
3. Из stdin: echo "console.log('Hello')" | ues-ds register my-handler --stdin

В скрипте доступны следующие объекты:
- event: объект события с полями type, key, value, timestamp, metadata
- console: для логирования (log, error, info)
- JSON: для работы с JSON (parse, stringify)
- Strings: утилиты для работы со строками
- Crypto: криптографические функции (md5, sha256)
- Time: работа со временем (now, format, parse)
- HTTP: HTTP запросы (если включен networking)

Примеры:
  ues-ds register logger "console.log('Event:', event.type, event.key)"
  ues-ds register webhook --file webhook.js --events put --events delete
  echo "console.log('Key updated:', event.key)" | ues-ds register updater --stdin`,
	})
}
