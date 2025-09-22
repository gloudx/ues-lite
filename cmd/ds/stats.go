package main

import (
	"context"
	"fmt"
	"os"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func stats(ctx *cli.Context) error {

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Подсчитываем ключи
	keysChan, errChan, err := app.ds.Keys(ctxTimeout, ds.NewKey("/"))
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
	kvChan, errChan2, err := app.ds.Iterator(ctxTimeout, ds.NewKey("/"), false)
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

func init() {
	commands = append(commands, &cli.Command{
		Name:   "stats",
		Usage:  "Показать статистику датастора",
		Action: stats,
	})
}
