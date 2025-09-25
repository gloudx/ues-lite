package main

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func listViews(ctx *cli.Context) error {
	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	views := app.ds.ListViews()

	if len(views) == 0 {
		fmt.Println("📋 Views не найдены")
		return nil
	}

	detailed := ctx.Bool("detailed")

	if detailed {
		// Подробный вывод
		for i, view := range views {
			if i > 0 {
				fmt.Println()
			}

			config := view.Config()
			stats := view.Stats()

			fmt.Printf("🔍 View: %s\n", config.ID)
			fmt.Printf("   Название: %s\n", config.Name)
			if config.Description != "" {
				fmt.Printf("   Описание: %s\n", config.Description)
			}
			fmt.Printf("   Источник: %s\n", config.SourcePrefix)
			if config.TargetPrefix != "" {
				fmt.Printf("   Цель: %s\n", config.TargetPrefix)
			}
			if config.FilterScript != "" {
				fmt.Printf("   Фильтр: %s\n", truncateString(config.FilterScript, 60))
			}
			if config.TransformScript != "" {
				fmt.Printf("   Трансформация: %s\n", truncateString(config.TransformScript, 60))
			}
			if config.SortScript != "" {
				fmt.Printf("   Сортировка: %s\n", truncateString(config.SortScript, 60))
			}

			fmt.Printf("   Кэширование: %v", config.EnableCaching)
			if config.EnableCaching {
				fmt.Printf(" (TTL: %v)", config.CacheTTL)
			}
			fmt.Println()

			fmt.Printf("   Автообновление: %v", config.AutoRefresh)
			if config.AutoRefresh {
				fmt.Printf(" (debounce: %v)", config.RefreshDebounce)
			}
			fmt.Println()

			if config.MaxResults > 0 {
				fmt.Printf("   Макс. результатов: %d\n", config.MaxResults)
			}

			fmt.Printf("   Создан: %s\n", config.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("   Обновлен: %s\n", config.UpdatedAt.Format("2006-01-02 15:04:05"))

			// Статистика
			if stats.RefreshCount > 0 {
				fmt.Printf("   Обновлений: %d\n", stats.RefreshCount)
				fmt.Printf("   Последнее обновление: %s\n", stats.LastRefresh.Format("2006-01-02 15:04:05"))
				fmt.Printf("   Результатов: %d\n", stats.ResultCount)
				if stats.ExecutionTimeMs > 0 {
					fmt.Printf("   Время выполнения: %dмс\n", stats.ExecutionTimeMs)
				}
				if stats.CacheHits > 0 || stats.CacheMisses > 0 {
					fmt.Printf("   Кэш: %d попаданий, %d промахов\n", stats.CacheHits, stats.CacheMisses)
				}
				if stats.ErrorCount > 0 {
					fmt.Printf("   Ошибок: %d\n", stats.ErrorCount)
					if stats.LastError != "" {
						fmt.Printf("   Последняя ошибка: %s\n", stats.LastError)
					}
				}
			}
		}
	} else {
		// Табличный вывод
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleColoredBright)
		t.SetTitle("📋 Список Views")

		t.AppendHeader(table.Row{"ID", "Название", "Источник", "Кэш", "Авто", "Результатов", "Обновлений", "Ошибок"})

		for _, view := range views {
			config := view.Config()
			stats := view.Stats()

			cache := "❌"
			if config.EnableCaching {
				cache = "✅"
			}

			autoRefresh := "❌"
			if config.AutoRefresh {
				autoRefresh = "✅"
			}

			t.AppendRow(table.Row{
				config.ID,
				truncateString(config.Name, 20),
				truncateString(config.SourcePrefix, 20),
				cache,
				autoRefresh,
				stats.ResultCount,
				stats.RefreshCount,
				stats.ErrorCount,
			})
		}

		t.Render()
	}

	fmt.Printf("\n📊 Всего views: %d\n", len(views))
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "list-views",
		Aliases: []string{"lv", "views"},
		Usage:   "Показать список всех views",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "detailed",
				Aliases: []string{"d"},
				Usage:   "Подробная информация о каждом view",
			},
		},
		Action: listViews,
		Description: `Показывает список всех созданных views с их конфигурацией и статистикой.

В обычном режиме выводится таблица с основной информацией.
В подробном режиме (-d/--detailed) выводится полная информация о каждом view:
- Конфигурация (фильтры, трансформации, настройки)
- Статистика выполнения
- Информация о кэше
- История ошибок

Примеры:
  # Краткий список
  ues-ds list-views
  
  # Подробная информация
  ues-ds list-views --detailed`,
	})
}
