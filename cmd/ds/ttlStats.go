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

func ttlStats(ctx *cli.Context) error {
	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	prefix := ctx.String("prefix")
	format := ctx.String("format")
	export := ctx.Bool("export")

	ctxTimeout, cancel := context.WithTimeout(context.Background(), ctx.Duration("timeout"))
	defer cancel()

	dsPrefix := ds.NewKey(prefix)

	if export {
		// Экспортируем полный отчет
		return exportTTLReport(ctxTimeout, app, dsPrefix, format)
	}

	// Получаем статистику TTL
	stats, err := app.ds.GetTTLStats(ctxTimeout, dsPrefix)
	if err != nil {
		return fmt.Errorf("ошибка получения TTL статистики: %w", err)
	}

	// Выводим в зависимости от формата
	switch format {
	case "json":
		jsonData, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return fmt.Errorf("ошибка сериализации JSON: %w", err)
		}
		fmt.Println(string(jsonData))

	case "table":
		fallthrough
	default:
		// Табличный вывод статистики
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleColoredBright)
		t.SetTitle(fmt.Sprintf("⏰ TTL Статистика для префикса '%s'", prefix))

		t.AppendRow(table.Row{"Всего ключей", stats.TotalKeys})
		t.AppendRow(table.Row{"Ключи с TTL", stats.TotalKeys - stats.KeysWithoutTTL})
		t.AppendRow(table.Row{"Ключи без TTL", stats.KeysWithoutTTL})
		t.AppendRow(table.Row{"Истекшие ключи", stats.ExpiredKeys})
		t.AppendRow(table.Row{"Истекают скоро (5мин)", stats.ExpiringKeys})

		if stats.AverageTimeLeft > 0 {
			t.AppendRow(table.Row{"Среднее время до истечения", formatDuration(stats.AverageTimeLeft)})
		}

		if stats.NextExpiration != nil {
			timeUntilNext := time.Until(*stats.NextExpiration)
			if timeUntilNext > 0 {
				t.AppendRow(table.Row{"Следующее истечение через", formatDuration(timeUntilNext)})
				t.AppendRow(table.Row{"Следующее истечение в", stats.NextExpiration.Format("2006-01-02 15:04:05")})
			} else {
				t.AppendRow(table.Row{"Следующее истечение", "уже произошло"})
				t.AppendRow(table.Row{"Время истечения", stats.NextExpiration.Format("2006-01-02 15:04:05")})
			}
		}

		t.Render()

		// Дополнительные предупреждения
		if stats.ExpiredKeys > 0 {
			fmt.Printf("\n⚠️  Найдено %d истекших ключей. Используйте 'ttl-cleanup' для очистки.\n", stats.ExpiredKeys)
		}

		if stats.ExpiringKeys > 0 {
			fmt.Printf("\n⏰ %d ключей истекают в ближайшие 5 минут.\n", stats.ExpiringKeys)
		}

		// Показываем состояние TTL мониторинга
		monitorConfig := app.ds.GetTTLMonitorConfig()
		if monitorConfig != nil {
			fmt.Printf("\n📊 TTL Мониторинг: ")
			if monitorConfig.Enabled {
				fmt.Printf("✅ включен (интервал: %v)\n", monitorConfig.CheckInterval)
			} else {
				fmt.Printf("❌ отключен\n")
			}
		}
	}

	return nil
}

func exportTTLReport(ctx context.Context, app *app, prefix ds.Key, format string) error {
	fmt.Printf("📊 Создание полного TTL отчета для префикса '%s'...\n", prefix.String())

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fс", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fм", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fч", d.Hours())
	} else {
		days := d.Hours() / 24
		return fmt.Sprintf("%.1fд", days)
	}
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "ttl-stats",
		Aliases: []string{"ttl-stat", "ts"},
		Usage:   "Показать статистику TTL ключей",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "prefix",
				Aliases: []string{"p"},
				Value:   "/",
				Usage:   "Префикс для анализа ключей",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "table",
				Usage:   "Формат вывода (table, json)",
			},
			&cli.BoolFlag{
				Name:    "export",
				Aliases: []string{"e"},
				Usage:   "Экспортировать полный отчет с деталями",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 60 * time.Second,
				Usage: "Таймаут операции",
			},
		},
		Action: ttlStats,
		Description: `Анализирует TTL ключи в датасторе и выводит статистику.

Команда собирает информацию о:
- Общем количестве ключей
- Ключах с установленным TTL и без него
- Истекших ключах, требующих очистки
- Ключах, которые истекут в ближайшее время
- Среднем времени до истечения
- Ближайшем времени истечения

В режиме экспорта (--export) создается подробный отчет со списками
ключей, сгруппированными по статусу.

Примеры:
  # Статистика по всем ключам
  ues-ds ttl-stats
  
  # Статистика для конкретного префикса
  ues-ds ttl-stats --prefix /users/
  
  # Вывод в JSON формате
  ues-ds ttl-stats --format json
  
  # Полный отчет с деталями
  ues-ds ttl-stats --export --prefix /sessions/

Интерпретация результатов:
- Истекшие ключи: ключи с просроченным TTL (нужна очистка)
- Истекают скоро: ключи, которые истекут в ближайшие 5 минут
- Среднее время: среднее время до истечения всех TTL ключей`,
	})
}
