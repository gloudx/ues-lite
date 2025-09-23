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

func queryJQ(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется jq выражение")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	jqQuery := ctx.Args().Get(0)
	mode := ctx.String("mode")
	prefix := ctx.String("prefix")
	output := ctx.String("output")
	pretty := ctx.Bool("pretty")

	// Подготавливаем опции для jq запроса
	opts := &datastore.JQQueryOptions{
		Prefix:           ds.NewKey(prefix),
		KeysOnly:         ctx.Bool("keys-only"),
		Limit:            ctx.Int("limit"),
		Timeout:          ctx.Duration("timeout"),
		TreatAsString:    ctx.Bool("treat-as-string"),
		IgnoreParseError: ctx.Bool("ignore-errors"),
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	fmt.Printf("🔍 Выполнение jq запроса: %s\n", jqQuery)
	fmt.Printf("📁 Префикс: %s\n", prefix)
	fmt.Printf("⚙️  Режим: %s\n", mode)

	switch mode {
	case "single":
		return executeJQSingle(ctxTimeout, app, jqQuery, ctx.String("key"), output, pretty)
	case "aggregate":
		return executeJQAggregate(ctxTimeout, app, jqQuery, opts, output, pretty)
	case "query":
		fallthrough
	default:
		return executeJQQuery(ctxTimeout, app, jqQuery, opts, output, pretty)
	}
}

func executeJQSingle(ctx context.Context, app *app, jqQuery, keyStr, output string, pretty bool) error {
	if keyStr == "" {
		return fmt.Errorf("для режима 'single' требуется указать ключ через --key")
	}

	key := ds.NewKey(keyStr)
	result, err := app.ds.QueryJQSingle(ctx, key, jqQuery)
	if err != nil {
		return fmt.Errorf("ошибка выполнения jq запроса: %w", err)
	}

	return outputResult(result, output, pretty, "single")
}

func executeJQAggregate(ctx context.Context, app *app, jqQuery string, opts *datastore.JQQueryOptions, output string, pretty bool) error {
	result, err := app.ds.AggregateJQ(ctx, jqQuery, opts)
	if err != nil {
		return fmt.Errorf("ошибка агрегации jq запроса: %w", err)
	}

	return outputResult(result, output, pretty, "aggregate")
}

func executeJQQuery(ctx context.Context, app *app, jqQuery string, opts *datastore.JQQueryOptions, output string, pretty bool) error {

	resultChan, errorChan, err := app.ds.QueryJQ(ctx, jqQuery, opts)
	if err != nil {
		return fmt.Errorf("ошибка создания jq запроса: %w", err)
	}

	var results []map[string]interface{}
	count := 0

	// Создаем таблицу для вывода
	t := table.NewWriter()
	if output == "" || output == "table" {
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleColoredBright)
		t.SetTitle("📋 Результаты jq запроса")
		t.AppendHeader(table.Row{"#", "Ключ", "Результат"})
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("таймаут выполнения запроса: %w", ctx.Err())

		case err, ok := <-errorChan:
			if ok && err != nil {
				return fmt.Errorf("ошибка при выполнении запроса: %w", err)
			}

		case result, ok := <-resultChan:

			if !ok {
				// Канал закрыт - выводим финальные результаты
				goto done
			}

			if result.Value != nil {

				count++

				if output == "" || output == "table" {
					// Форматируем результат для таблицы
					var resultStr string
					if result.Value != nil {
						if jsonBytes, err := json.Marshal(result.Value); err == nil {
							resultStr = string(jsonBytes)
							// Обрезаем слишком длинные результаты для таблицы
							if len(resultStr) > 100 {
								resultStr = resultStr[:97] + "..."
							}
						} else {
							resultStr = fmt.Sprintf("%v", result.Value)
						}
					} else {
						resultStr = "<null>"
					}
					t.AppendRow(table.Row{count, result.Key.String(), resultStr})
				} else {
					// Собираем результаты для JSON вывода
					resultMap := map[string]any{
						"key":   result.Key.String(),
						"value": result.Value,
					}
					results = append(results, resultMap)
				}
			}

			// Показываем прогресс для больших запросов
			if count%1000 == 0 && count > 0 {
				fmt.Printf("📈 Обработано: %d записей\n", count)
			}
		}
	}

done:
	// Выводим результаты
	if output == "" || output == "table" {
		if count > 0 {
			t.Render()
		}
		fmt.Printf("\n📊 Всего результатов: %d\n", count)
	} else {
		return outputResults(results, output, pretty)
	}

	return nil
}

func outputResult(result interface{}, output string, pretty bool, mode string) error {
	switch output {
	case "", "json":
		return outputJSON(result, pretty)
	case "raw":
		fmt.Printf("%v\n", result)
		return nil
	default:
		return fmt.Errorf("неподдерживаемый формат вывода: %s", output)
	}
}

func outputResults(results []map[string]interface{}, output string, pretty bool) error {
	switch output {
	case "json":
		return outputJSON(results, pretty)
	case "jsonl":
		return outputJSONL(results)
	case "raw":
		for _, result := range results {
			fmt.Printf("%v\n", result["value"])
		}
		return nil
	default:
		return fmt.Errorf("неподдерживаемый формат вывода: %s", output)
	}
}

func outputJSON(data interface{}, pretty bool) error {
	var output []byte
	var err error

	if pretty {
		output, err = json.MarshalIndent(data, "", "  ")
	} else {
		output, err = json.Marshal(data)
	}

	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func outputJSONL(results []map[string]interface{}) error {
	for _, result := range results {
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("ошибка сериализации JSONL: %w", err)
		}
		fmt.Println(string(output))
	}
	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "jq",
		Aliases: []string{"query"},
		Usage:   "Выполнить jq запрос к данным в датасторе",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "prefix",
				Aliases: []string{"p"},
				Value:   "/",
				Usage:   "Префикс для фильтрации ключей",
			},
			&cli.StringFlag{
				Name:    "mode",
				Aliases: []string{"m"},
				Value:   "query",
				Usage:   "Режим выполнения: 'query', 'aggregate', 'single'",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "Ключ для режима 'single'",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Формат вывода: 'table', 'json', 'jsonl', 'raw'",
			},
			&cli.BoolFlag{
				Name:    "pretty",
				Aliases: []string{"P"},
				Usage:   "Красивое форматирование JSON",
				Value:   true,
			},
			&cli.BoolFlag{
				Name:    "keys-only",
				Aliases: []string{"k"},
				Usage:   "Обрабатывать только ключи без значений",
			},
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"n"},
				Usage:   "Ограничить количество обрабатываемых записей",
			},
			&cli.DurationFlag{
				Name:    "timeout",
				Aliases: []string{"t"},
				Value:   60 * time.Second,
				Usage:   "Таймаут выполнения запроса",
			},
			&cli.BoolFlag{
				Name:  "treat-as-string",
				Usage: "Обрабатывать все значения как строки, а не JSON",
			},
			&cli.BoolFlag{
				Name:  "ignore-errors",
				Usage: "Игнорировать ошибки парсинга JSON и продолжать выполнение",
			},
		},
		Action:    queryJQ,
		ArgsUsage: "<jq-выражение>",
		Description: `Выполняет jq запросы к данным в датасторе с поддержкой трех режимов:

РЕЖИМЫ РАБОТЫ:
• query    - Потоковое выполнение jq над множеством записей (по умолчанию)
• aggregate - Агрегация всех результатов в один объект
• single   - Выполнение jq над одним ключом (требует --key)

ФОРМАТЫ ВЫВОДА:
• table - Табличный вывод (по умолчанию для режима query)  
• json  - JSON формат
• jsonl - JSON Lines (одна строка = один результат)
• raw   - Только значения без метаданных

jq ВЫРАЖЕНИЯ:
jq - мощный инструмент для обработки JSON данных. Поддерживает фильтрацию,
трансформацию, агрегацию и множество других операций.

ПРИМЕРЫ:

1. Базовая фильтрация:
   ues-ds jq 'select(.age > 21)' --prefix="/users"
   
2. Извлечение полей:
   ues-ds jq '{name: .name, email: .email}' --prefix="/users" --output=json
   
3. Агрегация данных:
   ues-ds jq 'group_by(.category) | map({category: .[0].category, count: length})'
   ues-ds jq --mode=aggregate 'map(.price) | add' --prefix="/products"
   
4. Работа с массивами:
   ues-ds jq '.items[] | select(.active == true)'
   ues-ds jq 'map(select(.status == "active")) | length'
   
5. Математические операции:
   ues-ds jq 'map(.amount) | add' --mode=aggregate --prefix="/transactions"
   ues-ds jq 'map(.price) | [min, max, (add / length)]' --mode=aggregate
   
6. Условная логика:
   ues-ds jq 'if .type == "premium" then .price * 0.9 else .price end'
   
7. Работа с ключами:
   ues-ds jq '. + {key_parts: (env.key | split("/"))}' --keys-only=false
   
8. Запрос к одному ключу:
   ues-ds jq '.user.profile' --mode=single --key="/users/john"
   
9. Обработка строковых значений:
   ues-ds jq 'split(",") | map(tonumber) | add' --treat-as-string --prefix="/csv-data"
   
10. Сложные трансформации:
    ues-ds jq 'group_by(.department) | map({
      dept: .[0].department, 
      employees: length, 
      avg_salary: (map(.salary) | add / length)
    })' --mode=aggregate
    
11. Фильтрация с условиями:
    ues-ds jq 'select(.created_at > "2024-01-01") | {id, name, created_at}'
    
12. Работа с датами и временем:
    ues-ds jq 'select(.timestamp > now - 86400) | .data'

ПОЛЕЗНЫЕ jq ФУНКЦИИ:
• select(условие) - фильтрация
• map(выражение) - преобразование массива
• group_by(поле) - группировка
• sort_by(поле) - сортировка  
• unique_by(поле) - уникальные значения
• min, max, add - агрегация
• length - количество элементов
• keys - ключи объекта
• has("ключ") - проверка наличия ключа
• empty - пустой результат (исключение из вывода)

ПРОИЗВОДИТЕЛЬНОСТЬ:
Для больших датасетов используйте --limit для ограничения количества записей
и --timeout для установки разумного таймаута выполнения.`,
	})
}
