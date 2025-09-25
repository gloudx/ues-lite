package main

import (
	"context"
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
)

func refreshView(ctx *cli.Context) error {
	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	all := ctx.Bool("all")
	invalidateCache := ctx.Bool("invalidate-cache")

	if all {
		return refreshAllViews(app, invalidateCache, ctx.Duration("timeout"))
	}

	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ID view или флаг --all")
	}

	viewID := ctx.Args().Get(0)

	// Проверяем существование view
	view, exists := app.ds.GetView(viewID)
	if !exists {
		return fmt.Errorf("view '%s' не найден", viewID)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), ctx.Duration("timeout"))
	defer cancel()

	// Инвалидируем кэш если требуется
	if invalidateCache {
		fmt.Printf("🗑️  Очистка кэша view '%s'...\n", viewID)
		if err := view.InvalidateCache(ctxTimeout); err != nil {
			fmt.Printf("⚠️  Ошибка очистки кэша: %v\n", err)
		}
	}

	// Получаем статистику до обновления
	statsBefore := view.Stats()

	fmt.Printf("🔄 Обновление view '%s'...\n", viewID)
	start := time.Now()

	err = view.Refresh(ctxTimeout)
	if err != nil {
		return fmt.Errorf("ошибка обновления view: %w", err)
	}

	duration := time.Since(start)

	// Получаем статистику после обновления
	statsAfter := view.Stats()

	fmt.Printf("✅ View '%s' успешно обновлен\n", viewID)
	fmt.Printf("   Время выполнения: %v\n", duration)
	fmt.Printf("   Результатов: %d\n", statsAfter.ResultCount)
	if statsAfter.ExecutionTimeMs > 0 {
		fmt.Printf("   Время выполнения view: %dмс\n", statsAfter.ExecutionTimeMs)
	}

	// Показываем изменения в статистике
	if statsAfter.RefreshCount > statsBefore.RefreshCount {
		fmt.Printf("   Обновлений: %d → %d\n", statsBefore.RefreshCount, statsAfter.RefreshCount)
	}

	if statsAfter.ErrorCount > statsBefore.ErrorCount {
		fmt.Printf("   ⚠️  Новых ошибок: %d\n", statsAfter.ErrorCount-statsBefore.ErrorCount)
		if statsAfter.LastError != "" {
			fmt.Printf("   Последняя ошибка: %s\n", statsAfter.LastError)
		}
	}

	return nil
}

func refreshAllViews(app *app, invalidateCache bool, timeout time.Duration) error {
	views := app.ds.ListViews()

	if len(views) == 0 {
		fmt.Println("📋 Views не найдены")
		return nil
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("🔄 Обновление всех views (%d)...\n", len(views))
	start := time.Now()

	successCount := 0
	errorCount := 0

	for _, view := range views {
		fmt.Printf("   🔄 %s... ", view.ID())

		// Инвалидируем кэш если требуется
		if invalidateCache {
			if err := view.InvalidateCache(ctxTimeout); err != nil {
				fmt.Printf("❌ (ошибка очистки кэша: %v)\n", err)
				errorCount++
				continue
			}
		}

		err := view.Refresh(ctxTimeout)
		if err != nil {
			fmt.Printf("❌ (ошибка: %v)\n", err)
			errorCount++
		} else {
			stats := view.Stats()
			fmt.Printf("✅ (%d результатов)\n", stats.ResultCount)
			successCount++
		}
	}

	duration := time.Since(start)

	fmt.Printf("\n📊 Результат обновления:\n")
	fmt.Printf("   Успешно: %d\n", successCount)
	if errorCount > 0 {
		fmt.Printf("   Ошибок: %d\n", errorCount)
	}
	fmt.Printf("   Общее время: %v\n", duration)

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "refresh-view",
		Aliases: []string{"refresh-views", "rv"},
		Usage:   "Обновить view(s)",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Обновить все views",
			},
			&cli.BoolFlag{
				Name:    "invalidate-cache",
				Aliases: []string{"i"},
				Usage:   "Очистить кэш перед обновлением",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 120 * time.Second,
				Usage: "Таймаут выполнения",
			},
		},
		Action:    refreshView,
		ArgsUsage: "[view-id]",
		Description: `Принудительно обновляет указанный view или все views.

Обновление view включает:
- Повторное выполнение всех скриптов (фильтр, трансформация, сортировка)
- Обработку всех записей из источника данных  
- Обновление кэшированных результатов
- Обновление статистики

При обновлении всех views (--all) операции выполняются последовательно,
и выводится сводная статистика по завершению.

Флаг --invalidate-cache очищает кэш перед обновлением, что гарантирует
полное перестроение результатов.

Примеры:
  # Обновить конкретный view
  ues-ds refresh-view user-profiles
  
  # Обновить с очисткой кэша
  ues-ds refresh-view active-users --invalidate-cache
  
  # Обновить все views
  ues-ds refresh-view --all
  
  # Обновить все views с увеличенным таймаутом
  ues-ds refresh-view --all --timeout 5m`,
	})
}
