package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"ues-lite/datastore"

	ds "github.com/ipfs/go-datastore"
	"github.com/itchyny/gojq"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/urfave/cli/v2"
)

// ExportRecord представляет запись для экспорта
type ExportRecord struct {
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Timestamp int64             `json:"timestamp,omitempty"`
	Size      int               `json:"size,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func exportJSONL(ctx *cli.Context) error {

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	// Параметры команды
	prefix := ctx.String("prefix")
	output := ctx.String("output")
	format := ctx.String("format")
	patch := ctx.StringSlice("patch")
	extract := ctx.String("extract")
	jqExpr := ctx.String("jq")
	limit := ctx.Int("limit")
	startKey := ctx.String("start")
	endKey := ctx.String("end")
	includeMetadata := ctx.Bool("metadata")
	skipSystem := ctx.Bool("skip-system")
	compress := ctx.Bool("compress")
	batchSize := ctx.Int("batch-size")

	// Компилируем jq выражение если указано
	var jqQuery *gojq.Query
	if jqExpr != "" {
		query, err := gojq.Parse(jqExpr)
		if err != nil {
			return fmt.Errorf("ошибка парсинга jq выражения '%s': %w", jqExpr, err)
		}
		jqQuery = query
		fmt.Printf("🔍 jq выражение: %s\n", jqExpr)
	}

	// Создаем контекст с таймаутом
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	fmt.Printf("📤 Экспорт данных в JSON Lines\n")
	fmt.Printf("🏷️  Префикс: %s\n", prefix)
	if output != "" {
		fmt.Printf("📄 Вывод: %s\n", output)
	} else {
		fmt.Printf("📄 Вывод: консоль\n")
	}
	fmt.Printf("📋 Формат: %s\n", format)

	// Получаем итератор
	dsPrefix := ds.NewKey(prefix)
	kvChan, errChan, err := app.ds.Iterator(ctxTimeout, dsPrefix, false)
	if err != nil {
		return fmt.Errorf("ошибка при создании итератора: %w", err)
	}

	// Создаем писатель
	writer, closer, err := createWriter(output, compress)
	if err != nil {
		return fmt.Errorf("ошибка при создании писателя: %w", err)
	}
	defer closer()

	exported := 0
	skipped := 0
	batch := make([]string, 0, batchSize)

	for {
		select {
		case <-ctxTimeout.Done():
			return fmt.Errorf("таймаут при экспорте: %w", ctxTimeout.Err())

		case kv, ok := <-kvChan:
			if !ok {
				// Записываем оставшийся batch
				if len(batch) > 0 {
					if err := writeBatch(writer, batch); err != nil {
						return fmt.Errorf("ошибка записи финального batch: %w", err)
					}
				}
				goto done
			}

			keyStr := kv.Key.String()

			// Проверяем диапазон ключей
			if startKey != "" && keyStr < startKey {
				skipped++
				continue
			}
			if endKey != "" && keyStr > endKey {
				// Достигли конца диапазона
				if len(batch) > 0 {
					if err := writeBatch(writer, batch); err != nil {
						return fmt.Errorf("ошибка записи batch: %w", err)
					}
				}
				goto done
			}

			// Пропускаем системные ключи если нужно
			if skipSystem && strings.HasPrefix(keyStr, "/_system/") {
				skipped++
				continue
			}

			// Проверяем лимит
			if limit > 0 && exported >= limit {
				if len(batch) > 0 {
					if err := writeBatch(writer, batch); err != nil {
						return fmt.Errorf("ошибка записи batch: %w", err)
					}
				}
				goto done
			}

			var jsonBytes []byte
			if format == "value-only" {
				jsonBytes = kv.Value
			} else {
				record, err := createExportRecord(kv, format, includeMetadata)
				if err != nil {
					fmt.Printf("⚠️  Ошибка при обработке ключа %s: %v\n", keyStr, err)
					skipped++
					continue
				}
				jsonBytes, err = json.Marshal(record)
				if err != nil {
					fmt.Printf("⚠️  Ошибка сериализации ключа %s: %v\n", keyStr, err)
					skipped++
					continue
				}
			}

			// Применяем extract
			if extract != "" {
				jsonBytes = []byte(gjson.GetBytes(jsonBytes, extract).String())
			}

			// Применяем patch операции
			if len(patch) > 0 {
				for _, p := range patch {
					var err error
					k, v, ok := strings.Cut(p, "=")
					if ok {
						var value any = v
						var t string
						t, v, ok = strings.Cut(v, "#")
						if ok {
							switch t {
							case "int":
								value = parseInt(v)
							case "float":
								value = parseFloat(v)
							case "bool":
								value = parseBool(v)
							case "json":
								var jsonVal any
								if err := json.Unmarshal([]byte(v), &jsonVal); err != nil {
									fmt.Printf("⚠️  Ошибка разбора JSON в патче '%s' для ключа %s: %v\n", p, keyStr, err)
									skipped++
									continue
								}
								value = jsonVal
							default:
								value = v // по умолчанию строка
							}
						}
						jsonBytes, err = sjson.SetBytes(jsonBytes, k, value)
						if err != nil {
							fmt.Printf("⚠️  Ошибка применения патча '%s' к ключу %s: %v\n", p, keyStr, err)
							skipped++
							continue
						}
					}
				}
			}

			// Применяем jq выражение
			if jqQuery != nil {
				transformedBytes, err := applyJQExpression(jqQuery, jsonBytes, keyStr)
				if err != nil {
					fmt.Printf("⚠️  Ошибка применения jq к ключу %s: %v\n", keyStr, err)
					skipped++
					continue
				}
				if string(transformedBytes) == "null" {
					skipped++
					continue
				}
				jsonBytes = transformedBytes
			}

			batch = append(batch, string(jsonBytes))
			exported++

			// Записываем batch если достигли размера
			if len(batch) >= batchSize {
				if err := writeBatch(writer, batch); err != nil {
					return fmt.Errorf("ошибка записи batch: %w", err)
				}
				batch = batch[:0] // Очищаем batch

				// Показываем прогресс
				if exported%10000 == 0 {
					fmt.Printf("📈 Экспортировано: %d записей\n", exported)
				}
			}

		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("ошибка при итерации: %w", err)
			}
		}
	}

done:
	fmt.Printf("\n✅ Экспорт завершён!\n")
	fmt.Printf("📊 Экспортировано: %d записей\n", exported)
	if skipped > 0 {
		fmt.Printf("⏭️  Пропущено: %d записей\n", skipped)
	}

	return nil
}

// applyJQExpression применяет jq выражение к JSON данным
func applyJQExpression(query *gojq.Query, jsonBytes []byte, keyStr string) ([]byte, error) {
	
	// Парсим JSON в interface{}
	var input interface{}
	if err := json.Unmarshal(jsonBytes, &input); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
	}

	// Применяем jq выражение
	iter := query.Run(input)

	// Получаем первый результат
	result, ok := iter.Next()
	if !ok {
		// Если нет результата, возвращаем null
		return []byte("null"), nil
	}

	// Проверяем на ошибку
	if err, ok := result.(error); ok {
		return nil, fmt.Errorf("ошибка выполнения jq: %w", err)
	}

	// Сериализуем результат обратно в JSON
	transformedBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации результата jq: %w", err)
	}

	return transformedBytes, nil
}

func createExportRecord(kv datastore.KeyValue, format string, includeMetadata bool) (interface{}, error) {

	keyStr := kv.Key.String()
	valueStr := string(kv.Value)

	switch format {
	case "simple":
		// Простой формат: только ключ и значение
		return map[string]string{
			"key":   keyStr,
			"value": valueStr,
		}, nil

	case "full":
		// Полный формат с метаданными
		record := ExportRecord{
			Key:       keyStr,
			Value:     valueStr,
			Timestamp: time.Now().Unix(),
			Size:      len(kv.Value),
		}

		if includeMetadata {
			record.Metadata = make(map[string]string)

			// Определяем тип контента
			if json.Valid(kv.Value) {
				record.Metadata["content_type"] = "json"
			} else if isUTF8(kv.Value) {
				record.Metadata["content_type"] = "text"
			} else {
				record.Metadata["content_type"] = "binary"
			}

			// Добавляем части ключа как метаданные
			parts := strings.Split(strings.Trim(keyStr, "/"), "/")
			for i, part := range parts {
				if part != "" {
					record.Metadata[fmt.Sprintf("key_part_%d", i)] = part
				}
			}
		}

		return record, nil

	case "value-only":
		// Только значения (для восстановления структуры)
		var jsonValue interface{}
		if json.Valid(kv.Value) {
			if err := json.Unmarshal(kv.Value, &jsonValue); err == nil {
				return jsonValue, nil
			}
		}
		return valueStr, nil

	default:
		return nil, fmt.Errorf("неизвестный формат: %s", format)
	}
}

func createWriter(output string, compress bool) (*bufio.Writer, func() error, error) {
	if output == "" || output == "-" {
		// Вывод в консоль
		return bufio.NewWriter(os.Stdout), func() error {
			return nil
		}, nil
	}

	// Создаем директорию если нужно
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("создание директории: %w", err)
	}

	// Определяем тип файла по расширению
	ext := strings.ToLower(filepath.Ext(output))

	switch ext {
	case ".zip":
		return createZipWriter(output)
	case ".gz":
		return createGzipWriter(output)
	default:
		// Обычный файл
		file, err := os.Create(output)
		if err != nil {
			return nil, nil, fmt.Errorf("создание файла: %w", err)
		}

		writer := bufio.NewWriter(file)
		closer := func() error {
			writer.Flush()
			return file.Close()
		}

		return writer, closer, nil
	}
}

func createGzipWriter(filename string) (*bufio.Writer, func() error, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, nil, err
	}

	gzipWriter := gzip.NewWriter(file)
	writer := bufio.NewWriter(gzipWriter)

	closer := func() error {
		writer.Flush()
		gzipWriter.Close()
		return file.Close()
	}

	return writer, closer, nil
}

func createZipWriter(filename string) (*bufio.Writer, func() error, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, nil, err
	}

	zipWriter := zip.NewWriter(file)

	// Создаем файл внутри архива
	baseName := strings.TrimSuffix(filepath.Base(filename), ".zip")
	if !strings.HasSuffix(baseName, ".jsonl") {
		baseName += ".jsonl"
	}

	zipFile, err := zipWriter.Create(baseName)
	if err != nil {
		zipWriter.Close()
		file.Close()
		return nil, nil, err
	}

	writer := bufio.NewWriter(zipFile)
	closer := func() error {
		writer.Flush()
		zipWriter.Close()
		return file.Close()
	}

	return writer, closer, nil
}

func writeBatch(writer *bufio.Writer, batch []string) error {
	for _, line := range batch {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// parseInt converts a string to int64, returns 0 on error
func parseInt(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// parseFloat converts a string to float64, returns 0.0 on error
func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}

// parseBool converts a string to bool, returns false on error
func parseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "export",
		Aliases: []string{"dump"},
		Usage:   "Экспортировать данные в JSON Lines формат",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "prefix",
				Aliases: []string{"p"},
				Value:   "/",
				Usage:   "Префикс для фильтрации ключей",
			},
			&cli.StringSliceFlag{
				Name:  "patch",
				Usage: "Патчи для JSON в формате 'path=value' или 'path=type#value' (type: int, float, bool, json)",
			},
			&cli.StringFlag{
				Name:  "extract",
				Usage: "JSONPath для извлечения части значения",
			},
			&cli.StringFlag{
				Name:  "jq",
				Usage: "jq выражение для фильтрации/трансформации данных",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Путь к выходному файлу (по умолчанию консоль). Поддерживает .jsonl, .gz, .zip",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "value-only",
				Usage:   "Формат экспорта: 'simple', 'full', 'value-only'",
			},
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"n"},
				Usage:   "Ограничить количество экспортируемых записей",
			},
			&cli.StringFlag{
				Name:  "start",
				Usage: "Начальный ключ для экспорта (включительно)",
			},
			&cli.StringFlag{
				Name:  "end",
				Usage: "Конечный ключ для экспорта (исключительно)",
			},
			&cli.BoolFlag{
				Name:    "metadata",
				Aliases: []string{"m"},
				Usage:   "Включить метаданные в экспорт (только для 'full' формата)",
			},
			&cli.BoolFlag{
				Name:  "skip-system",
				Usage: "Пропустить системные ключи (/_system/*)",
				Value: true,
			},
			&cli.BoolFlag{
				Name:    "compress",
				Aliases: []string{"z"},
				Usage:   "Использовать сжатие (автоматически для .gz/.zip)",
			},
			&cli.IntFlag{
				Name:    "batch-size",
				Aliases: []string{"b"},
				Value:   1000,
				Usage:   "Размер batch для записи",
			},
		},
		Action:    exportJSONL,
		ArgsUsage: " ",
		Description: `Экспортирует данные из датастора в JSON Lines формат.

Поддерживаемые форматы экспорта:
- simple: {"key": "...", "value": "..."} - минимальный формат
- full: полная информация включая размер, время, метаданные  
- value-only: только значения (полезно для восстановления данных)

Поддерживаемые выходные форматы:
- Консоль (по умолчанию)
- Обычные файлы (.jsonl)
- Сжатые файлы (.gz)
- ZIP архивы (.zip)

Обработка данных (применяется в порядке):
1. extract: извлечение части JSON по JSONPath
2. patch: изменение полей JSON
3. jq: фильтрация/трансформация с помощью jq выражений

Примеры:
  # Простой экспорт
  ues-ds export-jsonl --prefix="/logs" --output=logs.jsonl
  
  # С метаданными  
  ues-ds export --format=full --metadata -o backup.zip -n 10000
  
  # Фильтрация по диапазону ключей
  ues-ds dump --start="/user/a" --end="/user/z" --output=users.gz
  
  # Извлечение только значений
  ues-ds export-jsonl --prefix="/events" --format=value-only > events.json
  
  # Использование jq для фильтрации
  ues-ds export --jq 'select(.age > 18)' --format=full --prefix="/users"
  
  # Трансформация данных с jq
  ues-ds export --jq '{name: .name, email: .email}' --prefix="/users" -o users.jsonl
  
  # Фильтрация null значений
  ues-ds export --jq 'select(. != null)' --prefix="/data"
  
  # Комплексная обработка
  ues-ds export --extract=".user" --patch="active=bool#true" --jq 'select(.age > 21)' --prefix="/profiles"

jq выражения:
  select(.age > 18)          - фильтрация по условию
  {name, email}              - извлечение только нужных полей  
  .users[]                   - развертывание массива
  select(has("email"))       - проверка наличия поля
  map(select(.active))       - фильтрация массива
  select(. != null)          - исключение null значений`,
	})
}
