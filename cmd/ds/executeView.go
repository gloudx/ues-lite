package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func executeView(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ID view")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	viewID := ctx.Args().Get(0)
	useCache := ctx.Bool("cache")
	refresh := ctx.Bool("refresh")
	format := ctx.String("format")
	limit := ctx.Int("limit")

	// Проверяем существование view
	view, exists := app.ds.GetView(viewID)
	if !exists {
		return fmt.Errorf("view '%s' не найден", viewID)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), ctx.Duration("timeout"))
	defer cancel()

	// Обновляем view если требуется
	if refresh {
		fmt.Printf("🔄 Обновление view '%s'...\n", viewID)
		if err := view.Refresh(ctxTimeout); err != nil {
			return fmt.Errorf("ошибка обновления view: %w", err)
		}
	}

	var results []interface{}

	// Получаем результаты
	if useCache {
		fmt.Printf("💾 Попытка получить кэшированные результаты...\n")
		if cached, found, err := view.GetCached(ctxTimeout); err != nil {
			return fmt.Errorf("ошибка получения кэшированных результатов: %w", err)
		} else if found {
			fmt.Printf("✅ Использованы кэшированные результаты\n")
			for _, result := range cached {
				results = append(results, map[string]interface{}{
					"key":       result.Key.String(),
					"value":     result.Value,
					"score":     result.Score,
					"metadata":  result.Metadata,
					"timestamp": result.Timestamp.Format(time.RFC3339),
				})
			}
		} else {
			fmt.Printf("❌ Кэшированные результаты не найдены, выполняем view...\n")
			viewResults, err := view.Execute(ctxTimeout)
			if err != nil {
				return fmt.Errorf("ошибка выполнения view: %w", err)
			}
			for _, result := range viewResults {
				results = append(results, map[string]interface{}{
					"key":       result.Key.String(),
					"value":     result.Value,
					"score":     result.Score,
					"metadata":  result.Metadata,
					"timestamp": result.Timestamp.Format(time.RFC3339),
				})
			}
		}
	} else {
		fmt.Printf("🔍 Выполнение view '%s'...\n", viewID)
		viewResults, err := view.Execute(ctxTimeout)
		if err != nil {
			return fmt.Errorf("ошибка выполнения view: %w", err)
		}
		for _, result := range viewResults {
			results = append(results, map[string]interface{}{
				"key":       result.Key.String(),
				"value":     result.Value,
				"score":     result.Score,
				"metadata":  result.Metadata,
				"timestamp": result.Timestamp.Format(time.RFC3339),
			})
		}
	}

	// Применяем лимит если нужно
	if limit > 0 && len(results) > limit {
		results = results[:limit]
		fmt.Printf("⚠️  Результаты ограничены %d записями\n", limit)
	}

	// Выводим результаты в выбранном формате
	switch format {
	case "json":
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("ошибка сериализации JSON: %w", err)
		}
		fmt.Println(string(jsonData))

	case "jsonl":
		for _, result := range results {
			jsonData, err := json.Marshal(result)
			if err != nil {
				fmt.Printf("Ошибка сериализации записи: %v\n", err)
				continue
			}
			fmt.Println(string(jsonData))
		}

	case "table":
		fallthrough
	default:
		if len(results) == 0 {
			fmt.Println("📋 Результатов не найдено")
			return nil
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleColoredBright)
		t.SetTitle(fmt.Sprintf("📋 Результаты view '%s'", viewID))

		includeScore := ctx.Bool("include-score")
		includeTimestamp := ctx.Bool("include-timestamp")

		headers := []interface{}{"#", "Ключ", "Значение"}
		if includeScore {
			headers = append(headers, "Рейтинг")
		}
		if includeTimestamp {
			headers = append(headers, "Время")
		}
		t.AppendHeader(table.Row(headers))

		for i, result := range results {
			if resultMap, ok := result.(map[string]interface{}); ok {
				key := resultMap["key"].(string)
				value := fmt.Sprintf("%v", resultMap["value"])

				// Обрезаем длинные значения
				if len(value) > 100 {
					value = value[:97] + "..."
				}

				row := []interface{}{i + 1, key, value}

				if includeScore {
					score := resultMap["score"]
					row = append(row, fmt.Sprintf("%.2f", score))
				}

				if includeTimestamp {
					timestamp := resultMap["timestamp"].(string)
					if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
						row = append(row, t.Format("15:04:05"))
					} else {
						row = append(row, timestamp)
					}
				}

				t.AppendRow(table.Row(row))
			}
		}

		t.Render()
	}

	// Показываем статистику view
	stats := view.Stats()
	fmt.Printf("\n📊 Статистика view:\n")
	fmt.Printf("   Результатов: %d\n", len(results))
	fmt.Printf("   Всего обновлений: %d\n", stats.RefreshCount)
	if stats.ExecutionTimeMs > 0 {
		fmt.Printf("   Последнее время выполнения: %dмс\n", stats.ExecutionTimeMs)
	}
	if stats.CacheHits > 0 || stats.CacheMisses > 0 {
		fmt.Printf("   Кэш: %d попаданий, %d промахов\n", stats.CacheHits, stats.CacheMisses)
	}

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "execute-view",
		Aliases: []string{"exec-view", "ev"},
		Usage:   "Выполнить view и показать результаты",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "cache",
				Aliases: []string{"c"},
				Usage:   "Попытаться использовать кэшированные результаты",
			},
			&cli.BoolFlag{
				Name:    "refresh",
				Aliases: []string{"r"},
				Usage:   "Принудительно обновить перед выполнением",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "table",
				Usage:   "Формат вывода (table, json, jsonl)",
			},
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"n"},
				Usage:   "Ограничить количество результатов",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 60 * time.Second,
				Usage: "Таймаут выполнения",
			},
			&cli.BoolFlag{
				Name:  "include-score",
				Usage: "Включить рейтинг в табличный вывод",
			},
			&cli.BoolFlag{
				Name:  "include-timestamp",
				Usage: "Включить временные метки в табличный вывод",
			},
		},
		Action:    executeView,
		ArgsUsage: "<view-id>",
		Description: `Выполняет указанный view и выводит результаты.

View может быть выполнен с использованием кэшированных данных или 
с принудительным обновлением. Результаты могут быть выведены в 
различных форматах.

Форматы вывода:
- table: табличный вывод (по умолчанию)
- json: JSON с форматированием
- jsonl: JSON Lines (одна строка на результат)

Примеры:
  # Выполнить view с кэшем
  ues-ds execute-view active-users --cache
  
  # Принудительно обновить и выполнить
  ues-ds execute-view user-profiles --refresh
  
  # Вывод в JSON с лимитом
  ues-ds execute-view recent-posts --format json --limit 10
  
  # Табличный вывод с дополнительными колонками
  ues-ds execute-view top-products --include-score --include-timestamp`,
	})
}
