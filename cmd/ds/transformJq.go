package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
	"ues-lite/datastore"

	ds "github.com/ipfs/go-datastore"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func transformJq(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется JQ выражение")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	jqExpression := ctx.Args().Get(0)
	key := ctx.String("key")
	prefix := ctx.String("prefix")
	dryRun := ctx.Bool("dry-run")
	output := ctx.String("output")
	silent := ctx.Bool("silent")

	if silent {
		app.ds.SetSilentMode(true)
		defer app.ds.SetSilentMode(false)
	}

	// Подготавливаем опции трансформации
	opts := &datastore.TransformOptions{
		TreatAsString: ctx.Bool("treat-as-string"),
		IgnoreErrors:  ctx.Bool("ignore-errors"),
		DryRun:        dryRun,
		Timeout:       ctx.Duration("timeout"),
		BatchSize:     ctx.Int("batch-size"),
		JQExpression:  jqExpression,
	}

	// Устанавливаем префикс если указан
	if prefix != "" {
		opts.Prefix = ds.NewKey(prefix)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	var dsKey ds.Key
	if key != "" {
		dsKey = ds.NewKey(key)
	}

	fmt.Printf("🔄 JQ трансформация: %s\n", jqExpression)
	if key != "" {
		fmt.Printf("🔑 Ключ: %s\n", key)
	} else if prefix != "" {
		fmt.Printf("📁 Префикс: %s\n", prefix)
	}
	if dryRun {
		fmt.Printf("🔍 Режим: только просмотр (dry-run)\n")
	}

	start := time.Now()

	summary, err := app.ds.TransformWithJQ(ctxTimeout, dsKey, jqExpression, opts)
	if err != nil {
		return fmt.Errorf("ошибка трансформации: %w", err)
	}

	duration := time.Since(start)

	// Выводим результаты
	switch output {
	case "json":
		jsonData, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("ошибка сериализации JSON: %w", err)
		}
		fmt.Println(string(jsonData))

	case "table":
		fallthrough
	default:
		// Показываем сводку
		fmt.Printf("\n📊 Результат трансформации:\n")
		fmt.Printf("   Обработано: %d\n", summary.TotalProcessed)
		fmt.Printf("   Успешно: %d\n", summary.Successful)
		if summary.Errors > 0 {
			fmt.Printf("   Ошибок: %d\n", summary.Errors)
		}
		if summary.Skipped > 0 {
			fmt.Printf("   Пропущено: %d\n", summary.Skipped)
		}
		fmt.Printf("   Время выполнения: %v\n", duration)

		// Показываем детали если есть результаты и их немного
		if len(summary.Results) > 0 && len(summary.Results) <= 20 {
			fmt.Printf("\n📋 Детали трансформации:\n")

			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleColoredBright)

			t.AppendHeader(table.Row{"Ключ", "Статус", "Новое значение", "Ошибка"})

			for _, result := range summary.Results {
				status := "✅"
				errorMsg := ""

				if result.Error != nil {
					status = "❌"
					errorMsg = result.Error.Error()
					if len(errorMsg) > 50 {
						errorMsg = errorMsg[:47] + "..."
					}
				} else if result.Skipped {
					status = "⏭️"
					errorMsg = "пропущено"
				}

				newValue := ""
				if result.NewValue != nil {
					if jsonBytes, err := json.Marshal(result.NewValue); err == nil {
						newValue = string(jsonBytes)
						if len(newValue) > 80 {
							newValue = newValue[:77] + "..."
						}
					} else {
						newValue = fmt.Sprintf("%v", result.NewValue)
						if len(newValue) > 80 {
							newValue = newValue[:77] + "..."
						}
					}
				}

				keyStr := result.Key.String()
				if len(keyStr) > 30 {
					keyStr = keyStr[:27] + "..."
				}

				t.AppendRow(table.Row{keyStr, status, newValue, errorMsg})
			}

			t.Render()
		} else if len(summary.Results) > 20 {
			fmt.Printf("\n   (детали скрыты, всего записей: %d)\n", len(summary.Results))
		}
	}

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "transform-jq",
		Aliases: []string{"tjq", "transform"},
		Usage:   "Трансформировать данные с помощью JQ выражения",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "Трансформировать конкретный ключ",
			},
			&cli.StringFlag{
				Name:    "prefix",
				Aliases: []string{"p"},
				Value:   "/",
				Usage:   "Префикс для массовой трансформации",
			},
			&cli.BoolFlag{
				Name:    "dry-run",
				Aliases: []string{"n"},
				Usage:   "Только показать изменения, не применять",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "table",
				Usage:   "Формат вывода (table, json)",
			},
			&cli.BoolFlag{
				Name:  "treat-as-string",
				Usage: "Трактовать все значения как строки",
			},
			&cli.BoolFlag{
				Name:  "ignore-errors",
				Usage: "Игнорировать ошибки парсинга и продолжить",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 60 * time.Second,
				Usage: "Таймаут операции",
			},
			&cli.IntFlag{
				Name:    "batch-size",
				Aliases: []string{"b"},
				Value:   100,
				Usage:   "Размер батча для массовых операций",
			},
			&cli.BoolFlag{
				Name:  "silent",
				Usage: "Отключить публикацию событий для этой операции",
			},
		},
		Action:    transformJq,
		ArgsUsage: "<jq-выражение>",
		Description: `Трансформирует данные в датасторе с помощью JQ выражений.

JQ - мощный инструмент для обработки JSON данных, позволяющий:
- Фильтровать и отбирать данные
- Изменять структуру объектов  
- Выполнять вычисления и агрегации
- Комбинировать и трансформировать поля

Может работать с одним ключом (--key) или со всеми ключами по префиксу (--prefix).
По умолчанию работает в режиме dry-run, показывая что будет изменено.

Примеры JQ трансформаций:

1. Добавить новое поле:
   ues-ds transform-jq '. + {updated_at: now}' --prefix /users/

2. Переименовать поля:
   ues-ds transform-jq '{id: .user_id, name: .full_name, email}' --key /user/123

3. Вычислить новые значения:
   ues-ds transform-jq '.total = (.price * .quantity)' --prefix /orders/

4. Фильтровать и трансформировать:
   ues-ds transform-jq 'select(.active == true) | {name, email, last_login}' --prefix /users/

5. Работать с массивами:
   ues-ds transform-jq '.items |= map(select(.available == true))' --key /inventory

6. Числовые вычисления:
   ues-ds transform-jq '.score = (.points / .max_points * 100 | round)' --prefix /results/

Флаги:
  --dry-run: только показать изменения (по умолчанию)
  --treat-as-string: не парсить JSON, работать со строками
  --ignore-errors: пропускать записи с ошибками
  --batch-size: размер батча для производительности`,
	})
}
