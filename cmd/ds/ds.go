package main

import (
	"context"
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
	ds datastore.Datastore
}

func newApp(dataDir string) (*app, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("создание директории данных: %w", err)
	}
	ds, err := datastore.NewDatastorage(dataDir, &badger4.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("инициализация datastore: %w", err)
	}

	ds.CreateSimpleJSSubscription(context.Background(), "log_all", `
		console.log("zzzzzzzzzzzzzzzzzzzzzzzzz")
	`)

	return &app{ds: ds}, nil
}

func (app *app) Close() error {
	if app.ds != nil {
		return app.ds.Close()
	}
	return nil
}

func initApp(c *cli.Context) (*app, error) {
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
		},
		Commands: commands,
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
