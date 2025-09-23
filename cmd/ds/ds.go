package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"ues-lite/datastore"

	badger4 "github.com/ipfs/go-ds-badger4"
	"github.com/urfave/cli/v2"
)

const (
	DefaultDataDir = "./.data"
	AppName        = "ues-ds"
	AppVersion     = "1.0.0"
)

type app struct {
	ds       datastore.Datastore
	isRemote bool
}

func newApp(dataDir string) (*app, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("создание директории данных: %w", err)
	}
	ds, err := datastore.NewDatastorage(dataDir, &badger4.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("инициализация datastore: %w", err)
	}

	return &app{ds: ds}, nil
}

func (app *app) Close() error {
	if app.ds != nil {
		return app.ds.Close()
	}
	return nil
}

// func newRemoteApp(endpoint string) (*app, error) {
// 	ds, err := NewRemoteDatastore(endpoint)
// 	if err != nil {
// 		return nil, fmt.Errorf("подключение к удаленному датастору: %w", err)
// 	}

// 	// Создаем адаптер для RemoteDatastore
// 	adapter := &RemoteDatastoreAdapter{ds}

// 	return &app{ds: adapter, isRemote: true}, nil
// }

func initApp(c *cli.Context) (*app, error) {
	// Проверяем, указан ли эндпоинт для удаленного подключения
	// if endpoint := c.String("endpoint"); endpoint != "" {
	// 	fmt.Printf("🌐 Подключение к удаленному серверу: %s\n", endpoint)
	// 	return newRemoteApp(endpoint)
	// }

	// Локальный режим
	return newApp(c.String("data"))
}

var commands = []*cli.Command{}

func main() {
	app := &cli.App{
		Name:    AppName,
		Usage:   "Утилита для работы с ключами в датасторе",
		Version: AppVersion,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "data",
				Aliases: []string{"d"},
				Value:   DefaultDataDir,
				Usage:   "Директория для хранения данных",
				EnvVars: []string{"UES_DATA_DIR"},
			},
			&cli.StringFlag{
				Name:    "endpoint",
				Aliases: []string{"e"},
				Usage:   "Эндпоинт удаленного API сервера (http://host:port или unix:///path/to/socket)",
				EnvVars: []string{"UES_ENDPOINT"},
			},
		},
		Commands: commands,
		Before: func(c *cli.Context) error {
			// Валидация: нельзя использовать --data и --endpoint одновременно
			if c.String("data") != DefaultDataDir && c.String("endpoint") != "" {
				return fmt.Errorf("нельзя одновременно использовать --data и --endpoint")
			}
			return nil
		},
		Description: `UES Datastore - утилита для работы с ключ-значение хранилищем.

Поддерживает два режима работы:

1. ЛОКАЛЬНЫЙ РЕЖИМ (по умолчанию):
   ues-ds --data ./mydata list
   ues-ds put /test "Hello World"

2. УДАЛЕННЫЙ РЕЖИМ (через API):
   ues-ds --endpoint http://localhost:8080 list
   ues-ds --endpoint unix:///tmp/ues-ds.sock put /test "Hello World"

Переменные окружения:
   UES_DATA_DIR    - директория для локальных данных
   UES_ENDPOINT    - эндпоинт удаленного сервера

Примеры удаленных эндпоинтов:
   http://localhost:8080        # HTTP сервер
   https://myserver.com:8080    # HTTPS сервер  
   unix:///tmp/ues-ds.sock     # Unix socket

Для запуска собственного сервера используйте команду 'serve'.`,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d Б", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cБ", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func isUTF8(data []byte) bool {
	return string(data) == strings.ToValidUTF8(string(data), "")
}
