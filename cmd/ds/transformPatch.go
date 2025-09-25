package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"ues-lite/datastore"

	ds "github.com/ipfs/go-datastore"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func transformPatch(ctx *cli.Context) error {
	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	key := ctx.String("key")
	prefix := ctx.String("prefix")
	dryRun := ctx.Bool("dry-run")
	output := ctx.String("output")
	silent := ctx.Bool("silent")
	patchOpsStr := ctx.StringSlice("patch")

	if silent {
		app.ds.SetSilentMode(true)
		defer app.ds.SetSilentMode(false)
	}

	if len(patchOpsStr) == 0 {
		return fmt.Errorf("требуется хотя бы одна patch операция (--patch)")
	}

	// Парсим patch операции
	var patchOps []datastore.PatchOp
	for _, patchStr := range patchOpsStr {
		op, err := parsePatchOperation(patchStr)
		if err != nil {
			return fmt.Errorf("ошибка парсинга patch операции '%s': %w", patchStr, err)
		}
		patchOps = append(patchOps, op)
	}

	// Подготавливаем опции трансформации
	opts := &datastore.TransformOptions{
		TreatAsString:   ctx.Bool("treat-as-string"),
		IgnoreErrors:    ctx.Bool("ignore-errors"),
		DryRun:          dryRun,
		Timeout:         ctx.Duration("timeout"),
		BatchSize:       ctx.Int("batch-size"),
		PatchOperations: patchOps,
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

	fmt.Printf("🔄 JSON Patch трансформация:\n")
	for i, op := range patchOps {
		fmt.Printf("   %d. %s %s", i+1, op.Op, op.Path)
		if op.Value != nil {
			if jsonBytes, err := json.Marshal(op.Value); err == nil {
				fmt.Printf(" = %s", string(jsonBytes))
			}
		}
		fmt.Println()
	}

	if key != "" {
		fmt.Printf("🔑 Ключ: %s\n", key)
	} else if prefix != "" {
		fmt.Printf("📁 Префикс: %s\n", prefix)
	}
	if dryRun {
		fmt.Printf("🔍 Режим: только просмотр (dry-run)\n")
	}

	start := time.Now()

	summary, err := app.ds.TransformWithPatch(ctxTimeout, dsKey, patchOps, opts)
	if err != nil {
		return fmt.Errorf("ошибка patch трансформации: %w", err)
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
		fmt.Printf("\n📊 Результат patch трансформации:\n")
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

			t.AppendHeader(table.Row{"Ключ", "Статус", "Оригинал", "Результат", "Ошибка"})

			for _, result := range summary.Results {
				status := "✅"
				errorMsg := ""

				if result.Error != nil {
					status = "❌"
					errorMsg = result.Error.Error()
					if len(errorMsg) > 30 {
						errorMsg = errorMsg[:27] + "..."
					}
				} else if result.Skipped {
					status = "⏭️"
					errorMsg = "пропущено"
				}

				originalValue := ""
				newValue := ""

				if result.OriginalValue != nil {
					if jsonBytes, err := json.Marshal(result.OriginalValue); err == nil {
						originalValue = string(jsonBytes)
						if len(originalValue) > 50 {
							originalValue = originalValue[:47] + "..."
						}
					}
				}

				if result.NewValue != nil {
					if jsonBytes, err := json.Marshal(result.NewValue); err == nil {
						newValue = string(jsonBytes)
						if len(newValue) > 50 {
							newValue = newValue[:47] + "..."
						}
					}
				}

				keyStr := result.Key.String()
				if len(keyStr) > 25 {
					keyStr = keyStr[:22] + "..."
				}

				t.AppendRow(table.Row{keyStr, status, originalValue, newValue, errorMsg})
			}

			t.Render()
		} else if len(summary.Results) > 20 {
			fmt.Printf("\n   (детали скрыты, всего записей: %d)\n", len(summary.Results))
		}
	}

	return nil
}

func parsePatchOperation(patchStr string) (datastore.PatchOp, error) {
	// Формат: "op:path:value" или "op:path" для операций без значения
	parts := strings.SplitN(patchStr, ":", 3)

	if len(parts) < 2 {
		return datastore.PatchOp{}, fmt.Errorf("неверный формат patch операции, ожидается 'op:path' или 'op:path:value'")
	}

	op := datastore.PatchOp{
		Op:   strings.ToLower(parts[0]),
		Path: parts[1],
	}

	// Валидируем операцию
	validOps := map[string]bool{
		"replace": true,
		"add":     true,
		"remove":  true,
		"copy":    true,
		"move":    true,
		"test":    true,
	}

	if !validOps[op.Op] {
		return datastore.PatchOp{}, fmt.Errorf("неизвестная patch операция: %s", op.Op)
	}

	// Для операций remove обычно не нужно значение
	if op.Op == "remove" {
		return op, nil
	}

	// Для остальных операций нужно значение
	if len(parts) < 3 {
		return datastore.PatchOp{}, fmt.Errorf("операция %s требует значение", op.Op)
	}

	valueStr := parts[2]

	// Пытаемся разобрать как JSON, иначе используем как строку
	var value interface{}
	if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
		// Если не JSON, используем как строку
		value = valueStr
	}

	op.Value = value
	return op, nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "transform-patch",
		Aliases: []string{"tpatch", "patch"},
		Usage:   "Трансформировать данные с помощью JSON Patch операций",
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
			&cli.StringSliceFlag{
				Name:    "patch",
				Aliases: []string{"op"},
				Usage:   "Patch операция в формате 'op:path:value' (можно указать несколько)",
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
		Action:    transformPatch,
		ArgsUsage: " ",
		Description: `Трансформирует данные с помощью JSON Patch операций (RFC 6902).

JSON Patch позволяет точечно изменять JSON документы с помощью стандартных операций:
- replace: заменить значение поля
- add: добавить новое поле или элемент массива
- remove: удалить поле или элемент массива
- copy: скопировать значение из одного места в другое
- move: переместить значение
- test: проверить что значение равно ожидаемому

Формат patch операций: 'операция:путь:значение'
Путь использует JSON Pointer синтаксис (/field/subfield/0)

Примеры:

1. Заменить значение поля:
   ues-ds transform-patch --patch 'replace:/status:"active"' --key /user/123

2. Добавить новое поле:
   ues-ds transform-patch --patch 'add:/updated_at:"2025-01-01T00:00:00Z"' --prefix /users/

3. Удалить поле:
   ues-ds transform-patch --patch 'remove:/temporary_field' --prefix /data/

4. Несколько операций одновременно:
   ues-ds transform-patch \\
     --patch 'replace:/status:"inactive"' \\
     --patch 'add:/deactivated_at:"2025-01-01T00:00:00Z"' \\
     --patch 'remove:/session_token' \\
     --prefix /users/

5. Работа с массивами (добавить в конец):
   ues-ds transform-patch --patch 'add:/tags/-:"new-tag"' --key /post/456

6. Работа с числовыми значениями:
   ues-ds transform-patch --patch 'replace:/price:99.99' --key /product/789

Типы значений:
- Строки: "text" 
- Числа: 42, 3.14
- Булевы: true, false
- Null: null
- JSON объекты: {"key":"value"}
- JSON массивы: [1,2,3]

Флаги:
  --dry-run: только показать изменения (по умолчанию)
  --ignore-errors: пропускать записи с ошибками
  --batch-size: размер батча для производительности`,
	})
}
