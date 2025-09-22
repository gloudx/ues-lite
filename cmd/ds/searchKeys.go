package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func searchKeys(ctx *cli.Context) error {

	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется поисковая строка")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	searchStr := ctx.Args().Get(0)
	caseSensitive := ctx.Bool("case-sensitive")
	keysOnly := ctx.Bool("keys-only")
	limit := ctx.Int("limit")

	if !caseSensitive {
		searchStr = strings.ToLower(searchStr)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	kvChan, errChan, err := app.ds.Iterator(ctxTimeout, ds.NewKey("/"), keysOnly)
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

func init() {
	commands = append(commands, &cli.Command{
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
	})
}
