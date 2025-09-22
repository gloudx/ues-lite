package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

func unregisterHandler(ctx *cli.Context) error {

	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ID обработчика")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	id := ctx.Args().Get(0)
	force := ctx.Bool("force")

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Проверяем существование обработчика
	subscriptions, err := app.ds.ListJSSubscriptions(ctxTimeout)
	if err != nil {
		return fmt.Errorf("ошибка при проверке обработчиков: %w", err)
	}

	var foundSub *string
	for _, sub := range subscriptions {
		if sub.ID == id {
			foundSub = &sub.Script
			break
		}
	}

	if foundSub == nil {
		return fmt.Errorf("обработчик '%s' не найден", id)
	}

	// Запрашиваем подтверждение, если не указан force
	if !force {
		fmt.Printf("⚠️  Обработчик '%s' будет удален.\n", id)

		// Показываем превью скрипта
		scriptPreview := *foundSub
		if len(scriptPreview) > 200 {
			scriptPreview = scriptPreview[:197] + "..."
		}
		fmt.Printf("📄 Скрипт: %s\n\n", scriptPreview)

		fmt.Print("Вы уверены, что хотите удалить этот обработчик? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "да" {
			fmt.Println("❌ Операция отменена")
			return nil
		}
	}

	// Удаляем обработчик
	err = app.ds.RemoveJSSubscription(ctxTimeout, id)
	if err != nil {
		return fmt.Errorf("ошибка при удалении обработчика: %w", err)
	}

	fmt.Printf("🗑️  Обработчик '%s' успешно удален\n", id)

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "unsubscribe",
		Aliases: []string{"unreg", "unsub"},
		Usage:   "Отменить регистрацию обработчика событий",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Принудительное удаление без подтверждения",
			},
		},
		Action:    unregisterHandler,
		ArgsUsage: "<ID>",
		Description: `Удаляет зарегистрированный обработчик событий JavaScript.

Команда удаляет обработчик как из памяти, так и из постоянного хранилища.
По умолчанию запрашивает подтверждение и показывает превью скрипта.

Примеры:
  ues-ds unregister logger
  ues-ds unreg webhook --force
  ues-ds remove-handler my-handler`,
	})
}
