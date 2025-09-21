package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"ues-lite/datastore"

	ds "github.com/ipfs/go-datastore"
	badger4 "github.com/ipfs/go-ds-badger4"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

const (
	DefaultDataDir = "./.data"
	AppName        = "ds-cli"
	AppVersion     = "1.0.0"
)

func main() {
	app := &cli.App{
		Name:    AppName,
		Usage:   "Утилита для работы с ключами в датасторе",
		Version: AppVersion,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "data",
				Aliases: []string{"d"},
				Value:   DefaultDataDir,
				Usage:   "Директория для хранения данных",
				EnvVars: []string{"UES_DATA_DIR"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "put",
				Aliases: []string{"p"},
				Usage:   "Добавить или обновить ключ",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:    "ttl",
						Aliases: []string{"t"},
						Usage:   "Время жизни ключа (например: 1h, 30m, 60s)",
					},
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Сохранить значение как JSON",
					},
				},
				Action:    putKey,
				ArgsUsage: "<ключ> <значение>",
			},
			{
				Name:    "get",
				Aliases: []string{"g"},
				Usage:   "Получить значение по ключу",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Форматировать JSON вывод",
					},
				},
				Action:    getKey,
				ArgsUsage: "<ключ>",
			},
			{
				Name:      "delete",
				Aliases:   []string{"d", "del"},
				Usage:     "Удалить ключ",
				Action:    deleteKey,
				ArgsUsage: "<ключ>",
			},
			{
				Name:    "list",
				Aliases: []string{"l", "ls"},
				Usage:   "Перечислить ключи с префиксом",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "prefix",
						Aliases: []string{"p"},
						Value:   "/",
						Usage:   "Префикс для фильтрации ключей",
					},
					&cli.BoolFlag{
						Name:    "keys-only",
						Aliases: []string{"k"},
						Usage:   "Показать только ключи без значений",
					},
					&cli.IntFlag{
						Name:    "limit",
						Aliases: []string{"n"},
						Usage:   "Ограничить количество результатов",
					},
				},
				Action: listKeys,
			},
			{
				Name:    "search",
				Aliases: []string{"s"},
				Usage:   "Поиск ключей по подстроке",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "case-sensitive",
						Aliases: []string{"c"},
						Usage:   "Учитывать регистр при поиске",
					},
					&cli.BoolFlag{
						Name:    "keys-only",
						Aliases: []string{"k"},
						Usage:   "Показать только ключи без значений",
					},
					&cli.IntFlag{
						Name:    "limit",
						Aliases: []string{"n"},
						Usage:   "Ограничить количество результатов",
					},
				},
				Action:    searchKeys,
				ArgsUsage: "<поисковая строка>",
			},
			{
				Name:  "clear",
				Usage: "Очистить все ключи из датастора",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Принудительная очистка без подтверждения",
					},
				},
				Action: clearKeys,
			},
			{
				Name:      "info",
				Aliases:   []string{"i"},
				Usage:     "Показать информацию о ключе",
				Action:    keyInfo,
				ArgsUsage: "<ключ>",
			},
			{
				Name:   "stats",
				Usage:  "Показать статистику датастора",
				Action: stats,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func openDatastore(ctx *cli.Context) (datastore.Datastore, error) {
	path := ctx.String("data")
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию: %w", err)
	}
	opts := &badger4.DefaultOptions
	ds, err := datastore.NewDatastorage(path, opts)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть датастор: %w", err)
	}
	return ds, nil
}

func putKey(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		return fmt.Errorf("требуется ключ и значение")
	}
	key := ctx.Args().Get(0)
	value := ctx.Args().Get(1)
	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	dsKey := ds.NewKey(key)
	var data []byte
	if ctx.Bool("json") {
		var jsonData interface{}
		if err := json.Unmarshal([]byte(value), &jsonData); err != nil {
			return fmt.Errorf("неверный JSON: %w", err)
		}
		data, _ = json.Marshal(jsonData)
	} else {
		data = []byte(value)
	}
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ttl := ctx.Duration("ttl")
	if ttl > 0 {
		err = store.PutWithTTL(ctxTimeout, dsKey, data, ttl)
		fmt.Printf("✅ Ключ '%s' сохранён с TTL %v\n", key, ttl)
	} else {
		err = store.Put(ctxTimeout, dsKey, data)
		fmt.Printf("✅ Ключ '%s' сохранён\n", key)
	}
	return err
}

func getKey(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ключ")
	}
	key := ctx.Args().Get(0)
	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	dsKey := ds.NewKey(key)
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := store.Get(ctxTimeout, dsKey)
	if err != nil {
		if err == ds.ErrNotFound {
			return fmt.Errorf("ключ '%s' не найден", key)
		}
		return fmt.Errorf("ошибка при получении ключа: %w", err)
	}
	if ctx.Bool("json") {
		var jsonData interface{}
		if err := json.Unmarshal(data, &jsonData); err == nil {
			formatted, _ := json.MarshalIndent(jsonData, "", "  ")
			fmt.Println(string(formatted))
		} else {
			fmt.Println(string(data))
		}
	} else {
		fmt.Println(string(data))
	}
	return nil
}

func deleteKey(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ключ")
	}
	key := ctx.Args().Get(0)
	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	dsKey := ds.NewKey(key)
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = store.Delete(ctxTimeout, dsKey)
	if err != nil {
		return fmt.Errorf("ошибка при удалении ключа: %w", err)
	}
	fmt.Printf("🗑️  Ключ '%s' удалён\n", key)
	return nil
}

func listKeys(ctx *cli.Context) error {
	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	prefix := ctx.String("prefix")
	keysOnly := ctx.Bool("keys-only")
	limit := ctx.Int("limit")
	dsPrefix := ds.NewKey(prefix)
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	kvChan, errChan, err := store.Iterator(ctxTimeout, dsPrefix, keysOnly)
	if err != nil {
		return fmt.Errorf("ошибка при создании итератора: %w", err)
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	if keysOnly {
		t.AppendHeader(table.Row{"#", "Ключ"})
	} else {
		t.AppendHeader(table.Row{"#", "Ключ", "Значение", "Размер"})
	}
	count := 0
	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				goto done
			}
			count++
			if limit > 0 && count > limit {
				goto done
			}
			if keysOnly {
				t.AppendRow(table.Row{count, kv.Key.String()})
			} else {
				value := string(kv.Value)
				if len(value) > 100 {
					value = value[:97] + "..."
				}
				t.AppendRow(table.Row{count, kv.Key.String(), value, fmt.Sprintf("%d байт", len(kv.Value))})
			}
		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("ошибка при итерации: %w", err)
			}
		}
	}
done:
	if count == 0 {
		fmt.Printf("🔍 Ключи с префиксом '%s' не найдены\n", prefix)
		return nil
	}
	t.Render()
	fmt.Printf("\n📊 Найдено ключей: %d\n", count)
	return nil
}

func searchKeys(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется поисковая строка")
	}

	searchStr := ctx.Args().Get(0)
	caseSensitive := ctx.Bool("case-sensitive")
	keysOnly := ctx.Bool("keys-only")
	limit := ctx.Int("limit")

	if !caseSensitive {
		searchStr = strings.ToLower(searchStr)
	}

	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	kvChan, errChan, err := store.Iterator(ctxTimeout, ds.NewKey("/"), keysOnly)
	if err != nil {
		return fmt.Errorf("ошибка при создании итератора: %w", err)
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)

	if keysOnly {
		t.AppendHeader(table.Row{"#", "Ключ"})
	} else {
		t.AppendHeader(table.Row{"#", "Ключ", "Значение", "Размер"})
	}

	count := 0
	found := 0

	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				goto done
			}
			count++

			keyStr := kv.Key.String()
			searchKey := keyStr
			if !caseSensitive {
				searchKey = strings.ToLower(searchKey)
			}

			if strings.Contains(searchKey, searchStr) {
				found++

				if limit > 0 && found > limit {
					goto done
				}

				if keysOnly {
					t.AppendRow(table.Row{found, keyStr})
				} else {
					value := string(kv.Value)
					if len(value) > 100 {
						value = value[:97] + "..."
					}
					t.AppendRow(table.Row{found, keyStr, value, fmt.Sprintf("%d байт", len(kv.Value))})
				}
			}

		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("ошибка при итерации: %w", err)
			}
		}
	}

done:
	if found == 0 {
		fmt.Printf("🔍 Ключи содержащие '%s' не найдены (просмотрено %d ключей)\n", searchStr, count)
		return nil
	}

	t.Render()
	fmt.Printf("\n📊 Найдено: %d из %d ключей\n", found, count)
	return nil
}

func clearKeys(ctx *cli.Context) error {
	force := ctx.Bool("force")

	if !force {
		fmt.Print("⚠️  Вы уверены, что хотите удалить ВСЕ ключи? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "да" {
			fmt.Println("❌ Операция отменена")
			return nil
		}
	}

	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err = store.Clear(ctxTimeout)
	if err != nil {
		return fmt.Errorf("ошибка при очистке датастора: %w", err)
	}

	fmt.Println("🧹 Датастор очищен")
	return nil
}

func keyInfo(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ключ")
	}

	key := ctx.Args().Get(0)

	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	dsKey := ds.NewKey(key)

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Проверяем существование ключа
	exists, err := store.Has(ctxTimeout, dsKey)
	if err != nil {
		return fmt.Errorf("ошибка при проверке ключа: %w", err)
	}

	if !exists {
		fmt.Printf("❌ Ключ '%s' не существует\n", key)
		return nil
	}

	// Получаем значение
	data, err := store.Get(ctxTimeout, dsKey)
	if err != nil {
		return fmt.Errorf("ошибка при получении значения: %w", err)
	}

	// Получаем информацию о TTL
	expiration, err := store.GetExpiration(ctxTimeout, dsKey)
	var ttlInfo string
	if err != nil {
		ttlInfo = "Не установлен"
	} else if expiration.IsZero() {
		ttlInfo = "Не установлен"
	} else {
		remaining := time.Until(expiration)
		if remaining > 0 {
			ttlInfo = fmt.Sprintf("Истекает через %v (%s)", remaining, expiration.Format("2006-01-02 15:04:05"))
		} else {
			ttlInfo = "Истёк"
		}
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.SetTitle("ℹ️  Информация о ключе")

	t.AppendRow(table.Row{"Ключ", key})
	t.AppendRow(table.Row{"Размер значения", fmt.Sprintf("%d байт", len(data))})
	t.AppendRow(table.Row{"TTL", ttlInfo})

	// Пытаемся определить тип содержимого
	var contentType string
	if json.Valid(data) {
		contentType = "JSON"
	} else if isUTF8(data) {
		contentType = "Текст (UTF-8)"
	} else {
		contentType = "Бинарные данные"
	}
	t.AppendRow(table.Row{"Тип содержимого", contentType})

	t.Render()

	// Показываем превью значения
	fmt.Println("\n📄 Превью значения:")
	if len(data) > 500 {
		fmt.Printf("%s...\n[показано первые 500 из %d байт]\n", string(data[:500]), len(data))
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func stats(ctx *cli.Context) error {
	store, err := openDatastore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Подсчитываем ключи
	keysChan, errChan, err := store.Keys(ctxTimeout, ds.NewKey("/"))
	if err != nil {
		return fmt.Errorf("ошибка при получении ключей: %w", err)
	}

	totalKeys := 0
	for {
		select {
		case _, ok := <-keysChan:
			if !ok {
				goto countDone
			}
			totalKeys++
		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("ошибка при подсчёте ключей: %w", err)
			}
		}
	}

countDone:
	// Подсчитываем общий размер
	kvChan, errChan2, err := store.Iterator(ctxTimeout, ds.NewKey("/"), false)
	if err != nil {
		return fmt.Errorf("ошибка при создании итератора: %w", err)
	}

	var totalSize int64
	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				goto sizeDone
			}
			totalSize += int64(len(kv.Value))
		case err := <-errChan2:
			if err != nil {
				return fmt.Errorf("ошибка при подсчёте размера: %w", err)
			}
		}
	}

sizeDone:
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.SetTitle("📊 Статистика датастора")

	t.AppendRow(table.Row{"Путь", ctx.String("path")})
	t.AppendRow(table.Row{"Всего ключей", totalKeys})
	t.AppendRow(table.Row{"Общий размер значений", formatBytes(totalSize)})

	if totalKeys > 0 {
		avgSize := totalSize / int64(totalKeys)
		t.AppendRow(table.Row{"Средний размер значения", formatBytes(avgSize)})
	}

	t.Render()
	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d Б", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cБ", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func isUTF8(data []byte) bool {
	return string(data) == strings.ToValidUTF8(string(data), "")
}
