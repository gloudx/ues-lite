package main

import (
	"context"
	"fmt"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/urfave/cli/v2"
)

func deleteKey(ctx *cli.Context) error {

	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ключ")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	key := ctx.Args().Get(0)
	silent := ctx.Bool("silent")
	if silent {
		app.ds.SetSilentMode(true)
		defer app.ds.SetSilentMode(false)
	}

	dsKey := ds.NewKey(key)

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = app.ds.Delete(ctxTimeout, dsKey)
	if err != nil {
		return fmt.Errorf("ошибка при удалении ключа: %w", err)
	}

	fmt.Printf("🗑️  Ключ '%s' удалён\n", key)

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "delete",
		Aliases: []string{"d", "del"},
		Usage:   "Удалить ключ",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "silent",
				Usage: "Отключить публикацию событий для этой операции (только для этой команды)",
			},
		},
		Action:    deleteKey,
		ArgsUsage: "<ключ>",
	})
}
