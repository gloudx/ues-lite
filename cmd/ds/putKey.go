package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/urfave/cli/v2"
)

func putKey(ctx *cli.Context) error {

	if ctx.NArg() < 2 {
		return fmt.Errorf("требуется ключ и значение")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	key := ctx.Args().Get(0)
	value := ctx.Args().Get(1)

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
		err = app.ds.PutWithTTL(ctxTimeout, dsKey, data, ttl)
		fmt.Printf("✅ Ключ '%s' сохранён с TTL %v\n", key, ttl)
	} else {
		err = app.ds.Put(ctxTimeout, dsKey, data)
		fmt.Printf("✅ Ключ '%s' сохранён\n", key)
	}

	return err
}

func init() {
	commands = append(commands, &cli.Command{
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
	})
}
