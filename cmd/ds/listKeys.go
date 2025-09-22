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

func listKeys(ctx *cli.Context) error {

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	prefix := ctx.String("prefix")
	keysOnly := ctx.Bool("keys-only")
	limit := ctx.Int("limit")

	dsPrefix := ds.NewKey(prefix)

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	kvChan, errChan, err := app.ds.Iterator(ctxTimeout, dsPrefix, keysOnly)
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

func init() {
	commands = append(commands, &cli.Command{
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
	})
}
