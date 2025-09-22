package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func keyInfo(ctx *cli.Context) error {

	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ключ")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	key := ctx.Args().Get(0)

	dsKey := ds.NewKey(key)

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Проверяем существование ключа
	exists, err := app.ds.Has(ctxTimeout, dsKey)
	if err != nil {
		return fmt.Errorf("ошибка при проверке ключа: %w", err)
	}

	if !exists {
		fmt.Printf("❌ Ключ '%s' не существует\n", key)
		return nil
	}

	// Получаем значение
	data, err := app.ds.Get(ctxTimeout, dsKey)
	if err != nil {
		return fmt.Errorf("ошибка при получении значения: %w", err)
	}

	// Получаем информацию о TTL
	expiration, err := app.ds.GetExpiration(ctxTimeout, dsKey)
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

func init() {
	commands = append(commands, &cli.Command{
		Name:      "info",
		Aliases:   []string{"i"},
		Usage:     "Показать информацию о ключе",
		Action:    keyInfo,
		ArgsUsage: "<ключ>",
	})
}
