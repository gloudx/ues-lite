package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"ues-lite/blockstore"
	s "ues-lite/datastore"
	"ues-lite/helpers"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore/query"
	badger4 "github.com/ipfs/go-ds-badger4"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/multicodec"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

const (
	DefaultDataDir = "./.data"
	AppName        = "bs-cli"
	AppVersion     = "1.0.0"
)

type App struct {
	bs blockstore.Blockstore
	ds s.Datastore
}

func NewApp(dataDir string) (*App, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("создание директории данных: %w", err)
	}
	ds, err := s.NewDatastorage(dataDir, &badger4.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("инициализация datastore: %w", err)
	}
	bs := blockstore.NewBlockstore(ds)
	return &App{bs: bs, ds: ds}, nil
}

func (app *App) Close() error {
	if app.bs != nil {
		app.bs.Close()
	}
	if app.ds != nil {
		return app.ds.Close()
	}
	return nil
}

func main() {
	multicodec.RegisterEncoder(0x71, dagcbor.Encode)
	multicodec.RegisterDecoder(0x71, dagcbor.Decode)

	app := &cli.App{
		Name:     AppName,
		Version:  AppVersion,
		Usage:    "Утилита для работы с IPFS блокстором",
		Authors:  []*cli.Author{{Name: "Автор", Email: "author@example.com"}},
		Compiled: time.Now(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "data",
				Aliases: []string{"d"},
				Value:   DefaultDataDir,
				Usage:   "Директория для хранения данных",
				EnvVars: []string{"UES_DATA_DIR"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "put",
				Aliases: []string{"p"},
				Usage:   "Добавить данные в блокстор",
				Subcommands: []*cli.Command{
					putDataCommand(),
					putFileCommand(),
				},
			},
			{
				Name:    "get",
				Aliases: []string{"g"},
				Usage:   "Получить данные из блокстора",
				Subcommands: []*cli.Command{
					getDataCommand(),
					getFileCommand(),
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "Перечислить объекты в блоксторе",
				Action:  listAction,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "limit",
						Aliases: []string{"n"},
						Value:   50,
						Usage:   "Лимит объектов для вывода",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Подробный вывод",
					},
				},
			},
			{
				Name:    "search",
				Aliases: []string{"s"},
				Usage:   "Поиск объектов по CID или содержимому",
				Action:  searchAction,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "query",
						Aliases:  []string{"q"},
						Usage:    "Поисковый запрос",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "type",
						Aliases: []string{"t"},
						Value:   "all",
						Usage:   "Тип поиска: cid, content, all",
					},
				},
			},
			{
				Name:  "dag",
				Usage: "Работа с DAG структурами",
				Subcommands: []*cli.Command{
					dagShowCommand(),
					dagWalkCommand(),
					dagSubgraphCommand(),
				},
			},
			{
				Name:  "car",
				Usage: "Работа с CAR файлами",
				Subcommands: []*cli.Command{
					carExportCommand(),
					carImportCommand(),
				},
			},
			{
				Name:   "stats",
				Usage:  "Статистика блокстора",
				Action: statsAction,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "detailed",
						Aliases: []string{"d"},
						Usage:   "Подробная статистика",
					},
				},
			},
			{
				Name:   "prefetch",
				Usage:  "Предзагрузка данных",
				Action: prefetchAction,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "cid",
						Aliases:  []string{"c"},
						Usage:    "CID корневого объекта",
						Required: true,
					},
					&cli.IntFlag{
						Name:    "workers",
						Aliases: []string{"w"},
						Value:   8,
						Usage:   "Количество воркеров",
					},
				},
			},
		},
		Before: func(c *cli.Context) error {
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}

func putDataCommand() *cli.Command {
	return &cli.Command{
		Name:    "data",
		Aliases: []string{"d"},
		Usage:   "Добавить JSON данные",
		Action:  putDataAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "input",
				Aliases: []string{"i"},
				Usage:   "Входной JSON файл (или stdin)",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "json",
				Usage:   "Формат данных: json, cbor",
			},
		},
	}
}

func putFileCommand() *cli.Command {
	return &cli.Command{
		Name:    "file",
		Aliases: []string{"f"},
		Usage:   "Добавить файл",
		Action:  putFileAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "path",
				Aliases:  []string{"p"},
				Usage:    "Путь к файлу",
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "rabin",
				Aliases: []string{"r"},
				Usage:   "Использовать Rabin chunking",
			},
			&cli.BoolFlag{
				Name:  "progress",
				Usage: "Показать прогресс",
				Value: true,
			},
		},
	}
}

func getDataCommand() *cli.Command {
	return &cli.Command{
		Name:    "data",
		Aliases: []string{"d"},
		Usage:   "Получить данные как JSON",
		Action:  getDataAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cid",
				Aliases:  []string{"c"},
				Usage:    "CID объекта",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Выходной файл (или stdout)",
			},
			&cli.BoolFlag{
				Name:    "pretty",
				Aliases: []string{"p"},
				Usage:   "Красивое форматирование JSON",
				Value:   true,
			},
		},
	}
}

func getFileCommand() *cli.Command {
	return &cli.Command{
		Name:    "file",
		Aliases: []string{"f"},
		Usage:   "Получить файл",
		Action:  getFileAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cid",
				Aliases:  []string{"c"},
				Usage:    "CID файла",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Выходной файл или директория",
			},
		},
	}
}

func dagShowCommand() *cli.Command {
	return &cli.Command{
		Name:   "show",
		Usage:  "Показать структуру DAG",
		Action: dagShowAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cid",
				Aliases:  []string{"c"},
				Usage:    "CID корневого объекта",
				Required: true,
			},
			&cli.IntFlag{
				Name:    "depth",
				Aliases: []string{"d"},
				Value:   3,
				Usage:   "Максимальная глубина отображения",
			},
		},
	}
}

func dagWalkCommand() *cli.Command {
	return &cli.Command{
		Name:   "walk",
		Usage:  "Обойти весь DAG",
		Action: dagWalkAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cid",
				Aliases:  []string{"c"},
				Usage:    "CID корневого объекта",
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Подробный вывод",
			},
		},
	}
}

func dagSubgraphCommand() *cli.Command {
	return &cli.Command{
		Name:   "subgraph",
		Usage:  "Получить подграф DAG",
		Action: dagSubgraphAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cid",
				Aliases:  []string{"c"},
				Usage:    "CID корневого объекта",
				Required: true,
			},
		},
	}
}

func carExportCommand() *cli.Command {
	return &cli.Command{
		Name:   "export",
		Usage:  "Экспорт в CAR файл",
		Action: carExportAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cid",
				Aliases:  []string{"c"},
				Usage:    "CID корневого объекта",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "output",
				Aliases:  []string{"o"},
				Usage:    "Выходной CAR файл",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "progress",
				Usage: "Показать прогресс",
				Value: true,
			},
		},
	}
}

func carImportCommand() *cli.Command {
	return &cli.Command{
		Name:   "import",
		Usage:  "Импорт из CAR файла",
		Action: carImportAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "Входной CAR файл",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "progress",
				Usage: "Показать прогресс",
				Value: true,
			},
		},
	}
}

// Действия команд

func putDataAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()
	var reader io.Reader = os.Stdin
	inputFile := c.String("input")
	if inputFile != "" {
		file, err := os.Open(inputFile)
		if err != nil {
			return fmt.Errorf("открытие файла: %w", err)
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("чтение данных: %w", err)
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("парсинг JSON: %w", err)
	}
	node, err := helpers.ToNode(v)
	if err != nil {
		return fmt.Errorf("конвертация в IPLD узел: %w", err)
	}
	cid, err := app.bs.PutNode(c.Context, node)
	if err != nil {
		return fmt.Errorf("сохранение данных: %w", err)
	}
	fmt.Printf("✅ Данные добавлены: %s\n", cid.String())
	return nil
}

func putFileAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()
	filePath := c.String("path")
	useRabin := c.Bool("rabin")
	showProgress := c.Bool("progress")
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("открытие файла: %w", err)
	}
	defer file.Close()
	var reader io.Reader = file
	if showProgress {
		// stat, _ := file.Stat()
		// pw := progress.NewWriter()
		// pw.SetAutoStop(true)
		// pw.SetTrackerLength(25)
		// pw.SetMessageLength(24)
		// pw.SetStyle(progress.StyleDefault)
		// pw.SetUpdateFrequency(time.Millisecond * 100)
		// go pw.Render()
		// tracker := &progress.Tracker{
		// 	Message: filepath.Base(filePath),
		// 	Total:   stat.Size(),
		// 	Units:   progress.UnitsBytes,
		// }
		// pw.AppendTracker(tracker)
		// reader = progress.NewReader(file, tracker)
	}

	cid, err := app.bs.AddFile(c.Context, reader, useRabin)
	if err != nil {
		return fmt.Errorf("добавление файла: %w", err)
	}

	fmt.Printf("✅ Файл добавлен: %s\n", cid.String())
	return nil
}

func getDataAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	cidStr := c.String("cid")
	cid, err := cid.Parse(cidStr)
	if err != nil {
		return fmt.Errorf("неверный CID: %w", err)
	}

	node, err := app.bs.GetNode(c.Context, cid)
	if err != nil {
		return fmt.Errorf("получение данных: %w", err)
	}

	// Конвертация в JSON
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("сериализация в JSON: %w", err)
	}

	var output io.Writer = os.Stdout
	outputFile := c.String("output")
	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("создание файла: %w", err)
		}
		defer file.Close()
		output = file
	}

	if c.Bool("pretty") {
		var pretty interface{}
		json.Unmarshal(data, &pretty)
		prettyData, _ := json.MarshalIndent(pretty, "", "  ")
		data = prettyData
	}

	_, err = output.Write(data)
	return err
}

func getFileAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	cidStr := c.String("cid")
	cid, err := cid.Parse(cidStr)
	if err != nil {
		return fmt.Errorf("неверный CID: %w", err)
	}

	reader, err := app.bs.GetReader(c.Context, cid)
	if err != nil {
		return fmt.Errorf("получение файла: %w", err)
	}
	defer reader.Close()

	var output io.Writer = os.Stdout
	outputPath := c.String("output")
	if outputPath != "" {
		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("создание файла: %w", err)
		}
		defer file.Close()
		output = file
	}

	_, err = io.Copy(output, reader)
	if err != nil {
		return fmt.Errorf("копирование данных: %w", err)
	}

	if outputPath != "" {
		fmt.Printf("✅ Файл сохранен: %s\n", outputPath)
	}
	return nil
}

func listAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	limit := c.Int("limit")
	verbose := c.Bool("verbose")

	// Получаем все ключи из datastore
	query := query.Query{
		Limit: limit,
	}

	results, err := app.ds.Query(c.Context, query)
	if err != nil {
		return fmt.Errorf("запрос к datastore: %w", err)
	}
	defer results.Close()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)

	if verbose {
		t.AppendHeader(table.Row{"#", "CID", "Размер", "Тип"})
	} else {
		t.AppendHeader(table.Row{"#", "CID"})
	}

	count := 0
	for result := range results.Next() {
		if result.Error != nil {
			continue
		}

		count++
		cidStr := strings.TrimPrefix(result.Key, "/blocks/")

		if verbose {
			size := len(result.Value)
			t.AppendRow(table.Row{count, cidStr, formatBytes(size), "block"})
		} else {
			t.AppendRow(table.Row{count, cidStr})
		}
	}

	if count == 0 {
		fmt.Println("🔍 Блокстор пуст")
		return nil
	}

	t.AppendFooter(table.Row{"Всего", count})
	t.Render()
	return nil
}

func searchAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	q := c.String("query")
	searchType := c.String("type")

	fmt.Printf("🔍 Поиск: %s (тип: %s)\n", q, searchType)

	// Простой поиск по CID или содержимому
	dsQuery := query.Query{}
	results, err := app.ds.Query(c.Context, dsQuery)
	if err != nil {
		return fmt.Errorf("поиск: %w", err)
	}
	defer results.Close()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "CID", "Совпадение"})

	count := 0
	for result := range results.Next() {
		if result.Error != nil {
			continue
		}

		cidStr := strings.TrimPrefix(result.Key, "/blocks/")

		// Поиск по CID
		if searchType == "cid" || searchType == "all" {
			if strings.Contains(cidStr, q) {
				count++
				t.AppendRow(table.Row{count, cidStr, "CID"})
				continue
			}
		}

		// Поиск по содержимому (базовый)
		if searchType == "content" || searchType == "all" {
			if strings.Contains(string(result.Value), q) {
				count++
				t.AppendRow(table.Row{count, cidStr, "содержимое"})
			}
		}
	}

	if count == 0 {
		fmt.Println("❌ Ничего не найдено")
		return nil
	}

	t.AppendFooter(table.Row{"Найдено", count})
	t.Render()
	return nil
}

func dagShowAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	cidStr := c.String("cid")
	cid, err := cid.Parse(cidStr)
	if err != nil {
		return fmt.Errorf("неверный CID: %w", err)
	}

	depth := c.Int("depth")

	fmt.Printf("📊 DAG структура для %s (глубина: %d)\n\n", cid.String(), depth)

	currentDepth := 0
	err = app.bs.Walk(c.Context, cid, func(p traversal.Progress, n datamodel.Node) error {
		if currentDepth > depth {
			return nil
		}

		indent := strings.Repeat("  ", currentDepth)

		if p.LastBlock.Link != nil {
			fmt.Printf("%s├─ %s\n", indent, p.LastBlock.Link.String())
		} else {
			fmt.Printf("%s└─ (root)\n", indent)
		}

		currentDepth++
		return nil
	})

	if err != nil {
		return fmt.Errorf("обход DAG: %w", err)
	}

	return nil
}

func dagWalkAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	cidStr := c.String("cid")
	cid, err := cid.Parse(cidStr)
	if err != nil {
		return fmt.Errorf("неверный CID: %w", err)
	}

	verbose := c.Bool("verbose")

	fmt.Printf("🚶 Обход DAG для %s\n\n", cid.String())

	count := 0
	err = app.bs.Walk(c.Context, cid, func(p traversal.Progress, n datamodel.Node) error {

		count++

		if verbose {
			fmt.Printf("Шаг %d: путь=%s\n", count, p.Path.String())
			if p.LastBlock.Link != nil {
				fmt.Printf("  Ссылка: %s\n", p.LastBlock.Link.String())
			}
			fmt.Printf("  Узел: %s\n\n", n.Kind().String())
		} else {
			if p.LastBlock.Link != nil {
				fmt.Printf("%d. %s\n", count, p.LastBlock.Link.String())
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("обход DAG: %w", err)
	}

	fmt.Printf("\n✅ Обработано узлов: %d\n", count)
	return nil
}

func dagSubgraphAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	cidStr := c.String("cid")
	cid, err := cid.Parse(cidStr)
	if err != nil {
		return fmt.Errorf("неверный CID: %w", err)
	}

	selector := blockstore.BuildSelectorNodeExploreAll()
	cids, err := app.bs.GetSubgraph(c.Context, cid, selector)
	if err != nil {
		return fmt.Errorf("получение подграфа: %w", err)
	}

	fmt.Printf("📊 Подграф для %s\n\n", cid.String())

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "CID"})

	for i, c := range cids {
		t.AppendRow(table.Row{i + 1, c.String()})
	}

	t.AppendFooter(table.Row{"Всего", len(cids)})
	t.Render()
	return nil
}

func carExportAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	cidStr := c.String("cid")
	cid, err := cid.Parse(cidStr)
	if err != nil {
		return fmt.Errorf("неверный CID: %w", err)
	}

	outputPath := c.String("output")
	showProgress := c.Bool("progress")

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("создание файла: %w", err)
	}
	defer file.Close()

	var writer io.Writer = file
	if showProgress {
		pw := progress.NewWriter()
		pw.SetAutoStop(true)
		pw.SetTrackerLength(25)
		pw.SetMessageWidth(24)
		pw.SetStyle(progress.StyleDefault)
		pw.SetUpdateFrequency(time.Millisecond * 100)
		go pw.Render()

		tracker := &progress.Tracker{
			Message: "Экспорт CAR",
			Total:   100, // Примерное значение
			Units:   progress.UnitsDefault,
		}
		pw.AppendTracker(tracker)

		// Обертка для отслеживания прогресса
		writer = &progressWriter{writer: file, tracker: tracker}
	}

	selector := blockstore.BuildSelectorNodeExploreAll()
	err = app.bs.ExportCARV2(c.Context, cid, selector, writer)
	if err != nil {
		return fmt.Errorf("экспорт CAR: %w", err)
	}

	fmt.Printf("✅ CAR файл экспортирован: %s\n", outputPath)
	return nil
}

func carImportAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	inputPath := c.String("input")
	showProgress := c.Bool("progress")

	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("открытие файла: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file
	if showProgress {
		// stat, _ := file.Stat()
		// pw := progress.NewWriter()
		// pw.SetAutoStop(true)
		// pw.SetTrackerLength(25)
		// pw.SetMessageWidth(24)
		// pw.SetStyle(progress.StyleDefault)
		// pw.SetUpdateFrequency(time.Millisecond * 100)
		// go pw.Render()

		// tracker := &progress.Tracker{
		// 	Message: "Импорт CAR",
		// 	Total:   stat.Size(),
		// 	Units:   progress.UnitsBytes,
		// }
		// pw.AppendTracker(tracker)
		// reader = progress.NewReader(file, tracker)
	}

	roots, err := app.bs.ImportCARV2(c.Context, reader)
	if err != nil {
		return fmt.Errorf("импорт CAR: %w", err)
	}

	fmt.Printf("✅ CAR файл импортирован. Корневые CID:\n")
	for _, root := range roots {
		fmt.Printf("  - %s\n", root.String())
	}
	return nil
}

func statsAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	detailed := c.Bool("detailed")

	// Основная статистика
	q := query.Query{}
	results, err := app.ds.Query(c.Context, q)
	if err != nil {
		return fmt.Errorf("получение статистики: %w", err)
	}
	defer results.Close()

	var totalBlocks int64
	var totalSize int64
	var sizeDist = make(map[string]int)

	for result := range results.Next() {
		if result.Error != nil {
			continue
		}
		totalBlocks++
		size := int64(len(result.Value))
		totalSize += size

		// Распределение по размерам
		sizeCategory := categorizeSize(size)
		sizeDist[sizeCategory]++
	}

	// Вывод статистики
	fmt.Printf("📊 Статистика блокстора\n\n")

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"Метрика", "Значение"})

	t.AppendRow(table.Row{"Всего блоков", totalBlocks})
	t.AppendRow(table.Row{"Общий размер", formatBytes(int(totalSize))})

	if totalBlocks > 0 {
		avgSize := totalSize / totalBlocks
		t.AppendRow(table.Row{"Средний размер блока", formatBytes(int(avgSize))})
	}

	t.Render()

	if detailed && len(sizeDist) > 0 {
		fmt.Printf("\n📈 Распределение по размерам:\n\n")

		dt := table.NewWriter()
		dt.SetOutputMirror(os.Stdout)
		dt.SetStyle(table.StyleColoredBright)
		dt.AppendHeader(table.Row{"Категория размера", "Количество блоков"})

		for category, count := range sizeDist {
			dt.AppendRow(table.Row{category, count})
		}

		dt.Render()
	}

	return nil
}

func prefetchAction(c *cli.Context) error {
	app, err := initApp(c)
	if err != nil {
		return err
	}
	defer app.Close()

	cidStr := c.String("cid")
	cid, err := cid.Parse(cidStr)
	if err != nil {
		return fmt.Errorf("неверный CID: %w", err)
	}

	workers := c.Int("workers")

	fmt.Printf("⚡ Предзагрузка данных для %s (воркеры: %d)\n", cid.String(), workers)

	start := time.Now()
	selector := blockstore.BuildSelectorNodeExploreAll()
	err = app.bs.Prefetch(c.Context, cid, selector, workers)
	if err != nil {
		return fmt.Errorf("предзагрузка: %w", err)
	}

	duration := time.Since(start)
	fmt.Printf("✅ Предзагрузка завершена за %s\n", duration)
	return nil
}

// Вспомогательные функции

func initApp(c *cli.Context) (*App, error) {
	dataDir := c.String("data")
	return NewApp(dataDir)
}

func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func categorizeSize(size int64) string {
	switch {
	case size < 1024:
		return "< 1KB"
	case size < 1024*1024:
		return "1KB - 1MB"
	case size < 10*1024*1024:
		return "1MB - 10MB"
	case size < 100*1024*1024:
		return "10MB - 100MB"
	default:
		return "> 100MB"
	}
}

// progressWriter для отслеживания прогресса записи
type progressWriter struct {
	writer  io.Writer
	tracker *progress.Tracker
	written int64
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	pw.written += int64(n)
	pw.tracker.SetValue(pw.written)
	return
}
