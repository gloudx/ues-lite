package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

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

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	silent := ctx.Bool("silent")
	if silent {
		app.ds.SetSilentMode(true)
		defer app.ds.SetSilentMode(false)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err = app.ds.Clear(ctxTimeout)
	if err != nil {
		return fmt.Errorf("ошибка при очистке датастора: %w", err)
	}

	fmt.Println("🧹 Датастор очищен")

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:  "clear",
		Usage: "Очистить все ключи из датастора",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Принудительная очистка без подтверждения",
			},
			&cli.BoolFlag{
				Name:  "silent",
				Usage: "Отключить публикацию событий для этой операции (только для этой команды)",
			},
		},
		Action: clearKeys,
	})
}
