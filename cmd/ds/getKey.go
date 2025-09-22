package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/urfave/cli/v2"
)

func getKey(ctx *cli.Context) error {

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

	data, err := app.ds.Get(ctxTimeout, dsKey)
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

func init() {
	commands = append(commands, &cli.Command{
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
	})
}
