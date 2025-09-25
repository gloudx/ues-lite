package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

func deleteView(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется ID view")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	viewID := ctx.Args().Get(0)
	force := ctx.Bool("force")

	// Проверяем существование view
	view, exists := app.ds.GetView(viewID)
	if !exists {
		return fmt.Errorf("view '%s' не найден", viewID)
	}

	config := view.Config()
	stats := view.Stats()

	// Запрашиваем подтверждение, если не указан force
	if !force {
		fmt.Printf("⚠️  View '%s' будет удален.\n", viewID)
		fmt.Printf("   Название: %s\n", config.Name)
		if config.Description != "" {
			fmt.Printf("   Описание: %s\n", config.Description)
		}
		fmt.Printf("   Источник: %s\n", config.SourcePrefix)
		fmt.Printf("   Создан: %s\n", config.CreatedAt.Format("2006-01-02 15:04:05"))
		if stats.RefreshCount > 0 {
			fmt.Printf("   Обновлений: %d\n", stats.RefreshCount)
			fmt.Printf("   Результатов: %d\n", stats.ResultCount)
		}
		fmt.Println()

		fmt.Print("Вы уверены, что хотите удалить этот view? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "да" {
			fmt.Println("❌ Операция отменена")
			return nil
		}
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Удаляем view
	err = app.ds.RemoveView(ctxTimeout, viewID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении view: %w", err)
	}

	fmt.Printf("🗑️  View '%s' успешно удален\n", viewID)

	// Информируем о том, что было удалено
	fmt.Println("   ✅ Конфигурация удалена")
	if config.EnableCaching {
		fmt.Println("   ✅ Кэш очищен")
	}
	fmt.Println("   ✅ Статистика удалена")

	return nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "delete-view",
		Aliases: []string{"dv", "remove-view", "rm-view"},
		Usage:   "Удалить view",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Принудительное удаление без подтверждения",
			},
		},
		Action:    deleteView,
		ArgsUsage: "<view-id>",
		Description: `Удаляет указанный view со всеми его данными.

При удалении view удаляются:
- Конфигурация view (фильтры, трансформации, настройки)
- Кэшированные результаты (если кэширование было включено)
- Статистика выполнения
- Подписки на автоматическое обновление

По умолчанию команда запрашивает подтверждение и показывает 
информацию о view перед удалением.

Примеры:
  # Удалить с подтверждением
  ues-ds delete-view user-profiles
  
  # Принудительное удаление
  ues-ds delete-view old-view --force
  
  # Удаление с псевдонимом команды
  ues-ds rm-view temporary-view -f

⚠️  Внимание: Удаление view необратимо!`,
	})
}
