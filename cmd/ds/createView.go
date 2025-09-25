package main

import (
	"context"
	"fmt"
	"time"
	"ues-lite/datastore"

	"github.com/urfave/cli/v2"
)

func createView(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		return fmt.Errorf("требуется ID и источник данных")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	id := ctx.Args().Get(0)
	sourcePrefix := ctx.Args().Get(1)
	name := ctx.String("name")
	if name == "" {
		name = id
	}

	// Создаем конфигурацию view
	config := datastore.ViewConfig{
		ID:              id,
		Name:            name,
		Description:     ctx.String("description"),
		SourcePrefix:    sourcePrefix,
		TargetPrefix:    ctx.String("target-prefix"),
		FilterScript:    ctx.String("filter"),
		TransformScript: ctx.String("transform"),
		SortScript:      ctx.String("sort"),
		StartKey:        ctx.String("start-key"),
		EndKey:          ctx.String("end-key"),
		EnableCaching:   ctx.Bool("cache"),
		CacheTTL:        ctx.Duration("cache-ttl"),
		AutoRefresh:     ctx.Bool("auto-refresh"),
		RefreshDebounce: ctx.Duration("debounce"),
		MaxResults:      ctx.Int("max-results"),
	}

	// Если target prefix не указан, генерируем автоматически
	if config.TargetPrefix == "" {
		config.TargetPrefix = fmt.Sprintf("/views/%s", id)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	view, err := app.ds.CreateView(ctxTimeout, config)
	if err != nil {
		return fmt.Errorf("ошибка создания view: %w", err)
	}

	fmt.Printf("✅ View '%s' создан\n", view.ID())
	fmt.Printf("📁 Источник: %s\n", config.SourcePrefix)
	if config.TargetPrefix != "" {
		fmt.Printf("🎯 Цель: %s\n", config.TargetPrefix)
	}
	if config.FilterScript != "" {
		fmt.Printf("🔍 Фильтр: %s\n", config.FilterScript)
	}
	if config.TransformScript != "" {
		fmt.Printf("🔄 Трансформация: %s\n", config.TransformScript)
	}
	if config.SortScript != "" {
		fmt.Printf("📊 Сортировка: %s\n", config.SortScript)
	}
	if config.EnableCaching {
		fmt.Printf("💾 Кэширование: включено (TTL: %v)\n", config.CacheTTL)
	}
	if config.AutoRefresh {
		fmt.Printf("🔄 Автообновление: включено (debounce: %v)\n", config.RefreshDebounce)
	}

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "create-view",
		Aliases: []string{"cv"},
		Usage:   "Создать новый view",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Имя view (по умолчанию ID)",
			},
			&cli.StringFlag{
				Name:    "description",
				Aliases: []string{"desc"},
				Usage:   "Описание view",
			},
			&cli.StringFlag{
				Name:  "target-prefix",
				Usage: "Префикс для результатов view",
			},
			&cli.StringFlag{
				Name:    "filter",
				Aliases: []string{"f"},
				Usage:   "JavaScript скрипт фильтрации",
			},
			&cli.StringFlag{
				Name:    "transform",
				Aliases: []string{"t"},
				Usage:   "JavaScript скрипт трансформации",
			},
			&cli.StringFlag{
				Name:    "sort",
				Aliases: []string{"s"},
				Usage:   "JavaScript скрипт сортировки",
			},
			&cli.StringFlag{
				Name:  "start-key",
				Usage: "Начальный ключ диапазона",
			},
			&cli.StringFlag{
				Name:  "end-key",
				Usage: "Конечный ключ диапазона",
			},
			&cli.BoolFlag{
				Name:  "cache",
				Value: true,
				Usage: "Включить кэширование результатов",
			},
			&cli.DurationFlag{
				Name:  "cache-ttl",
				Value: 10 * time.Minute,
				Usage: "Время жизни кэша",
			},
			&cli.BoolFlag{
				Name:  "auto-refresh",
				Value: true,
				Usage: "Автоматическое обновление при изменении данных",
			},
			&cli.DurationFlag{
				Name:  "debounce",
				Value: 2 * time.Second,
				Usage: "Задержка группировки обновлений",
			},
			&cli.IntFlag{
				Name:  "max-results",
				Value: 1000,
				Usage: "Максимальное количество результатов",
			},
		},
		Action:    createView,
		ArgsUsage: "<view-id> <source-prefix>",
		Description: `Создает новый view для фильтрации, трансформации и кэширования данных.

View позволяет создавать виртуальные представления данных с помощью JavaScript:
- Фильтрация: отбор нужных записей
- Трансформация: изменение структуры данных  
- Сортировка: упорядочивание результатов
- Кэширование: сохранение результатов в памяти

Примеры:

1. Простой view для активных пользователей:
   ues-ds create-view active-users /users/ \
     --filter "data.json && data.json.active === true"

2. View с трансформацией данных:
   ues-ds create-view user-profiles /users/ \
     --filter "data.json && data.json.active" \
     --transform "return {name: data.json.name, email: data.json.email};"

3. View с сортировкой по дате:
   ues-ds create-view recent-posts /posts/ \
     --sort "return new Date(data.json.created_at).getTime();"

4. View с ограничением диапазона:
   ues-ds create-view orders-2024 /orders/ \
     --start-key "/orders/2024-01-01" \
     --end-key "/orders/2025-01-01"

JavaScript API в скриптах:
- data.key - ключ записи
- data.value - сырое значение (строка)
- data.json - разобранный JSON (если валидный)
- data.size - размер значения в байтах

Функции должны возвращать:
- filter: true/false для включения записи
- transform: новый объект данных
- sort: числовое значение для сортировки`,
	})
}
