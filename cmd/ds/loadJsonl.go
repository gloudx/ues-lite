package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"ues-lite/tid"

	ds "github.com/ipfs/go-datastore"
	"github.com/urfave/cli/v2"
)

func loadJSONL(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("требуется путь к файлу или архиву")
	}

	app, err := initApp(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	sourcePath := ctx.Args().Get(0)
	prefix := ctx.String("prefix")
	idType := ctx.String("id-type")
	batchSize := ctx.Int("batch-size")
	clockID := ctx.Uint("clock-id")

	// Создаем TID clock если нужен TID
	var tidClock tid.TIDClock
	if idType == "tid" {
		tidClock = tid.NewTIDClock(clockID)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	fmt.Printf("📥 Загрузка JSON Lines из: %s\n", sourcePath)
	fmt.Printf("🏷️  Префикс ключей: %s\n", prefix)
	fmt.Printf("🆔 Тип ID: %s\n", idType)

	// Определяем тип источника
	readers, err := getReaders(sourcePath)
	if err != nil {
		return fmt.Errorf("ошибка при открытии источника: %w", err)
	}

	totalProcessed := 0
	totalErrors := 0

	// Обрабатываем каждый файл
	for filename, reader := range readers {
		fmt.Printf("\n📄 Обработка файла: %s\n", filename)

		processed, errors, err := processJSONLFile(ctxTimeout, app, reader, prefix, idType, &tidClock, batchSize)
		if err != nil {
			fmt.Printf("❌ Ошибка при обработке файла %s: %v\n", filename, err)
			continue
		}

		totalProcessed += processed
		totalErrors += errors

		fmt.Printf("✅ Обработано строк: %d, ошибок: %d\n", processed, errors)
	}

	fmt.Printf("\n📊 Итого загружено: %d записей\n", totalProcessed)
	if totalErrors > 0 {
		fmt.Printf("⚠️  Всего ошибок: %d\n", totalErrors)
	}

	return nil
}

func getReaders(sourcePath string) (map[string]io.Reader, error) {
	readers := make(map[string]io.Reader)

	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close() // Закрываем файл в конце функции

	// Получаем информацию о файле
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Если это директория - обрабатываем все .jsonl файлы
	if info.IsDir() {
		return getReadersFromDir(sourcePath)
	}

	ext := strings.ToLower(filepath.Ext(sourcePath))
	baseName := filepath.Base(sourcePath)

	switch ext {
	case ".zip":
		return getReadersFromZip(file)

	case ".tar", ".tgz":
		return getReadersFromTar(file, false)

	case ".gz":
		if strings.HasSuffix(strings.ToLower(baseName), ".tar.gz") {
			return getReadersFromTar(file, true)
		} else {
			// Обычный gzip файл - читаем в память
			gzipReader, err := gzip.NewReader(file)
			if err != nil {
				return nil, err
			}
			defer gzipReader.Close()

			data, err := io.ReadAll(gzipReader)
			if err != nil {
				return nil, err
			}
			readers[baseName] = strings.NewReader(string(data))
		}

	case ".jsonl", ".ndjson", ".json":
		// Читаем файл в память
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		readers[baseName] = strings.NewReader(string(data))

	default:
		// Пробуем как обычный текстовый файл - читаем в память
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		readers[baseName] = strings.NewReader(string(data))
	}

	return readers, nil
}

func getReadersFromDir(dirPath string) (map[string]io.Reader, error) {
	readers := make(map[string]io.Reader)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".jsonl" || ext == ".ndjson" || ext == ".json" {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				// Читаем файл в память
				data, err := io.ReadAll(file)
				if err != nil {
					return err
				}

				relPath, _ := filepath.Rel(dirPath, path)
				readers[relPath] = strings.NewReader(string(data))
			}
		}

		return nil
	})

	return readers, err
}

func getReadersFromZip(file *os.File) (map[string]io.Reader, error) {
	readers := make(map[string]io.Reader)

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}

	for _, zipFile := range zipReader.File {
		if !zipFile.FileInfo().IsDir() {
			ext := strings.ToLower(filepath.Ext(zipFile.Name))
			if ext == ".jsonl" || ext == ".ndjson" || ext == ".json" {
				rc, err := zipFile.Open()
				if err != nil {
					continue
				}

				// Читаем содержимое в память
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					continue
				}

				readers[zipFile.Name] = strings.NewReader(string(data))
			}
		}
	}

	return readers, nil
}

func getReadersFromTar(file *os.File, isGzip bool) (map[string]io.Reader, error) {
	readers := make(map[string]io.Reader)

	var tarReader *tar.Reader
	if isGzip {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		tarReader = tar.NewReader(gzipReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Typeflag == tar.TypeReg {
			ext := strings.ToLower(filepath.Ext(header.Name))
			if ext == ".jsonl" || ext == ".ndjson" || ext == ".json" {
				// Читаем весь файл в память (так как tar reader последовательный)
				data, err := io.ReadAll(tarReader)
				if err != nil {
					continue
				}
				readers[header.Name] = strings.NewReader(string(data))
			}
		}
	}

	return readers, nil
}

func processJSONLFile(ctx context.Context, app *app, reader io.Reader, prefix, idType string, tidClock *tid.TIDClock, batchSize int) (int, int, error) {
	scanner := bufio.NewScanner(reader)

	// Увеличиваем буфер для больших строк
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line size

	batch, err := app.ds.Batch(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("ошибка создания batch: %w", err)
	}

	processed := 0
	errors := 0
	batchCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Проверяем валидность JSON
		var jsonData interface{}
		if err := json.Unmarshal([]byte(line), &jsonData); err != nil {
			errors++
			continue
		}

		// Генерируем ID
		var keyID string
		switch idType {
		case "tid":
			keyID = tidClock.Next().String()
		case "unix-nano":
			keyID = strconv.FormatInt(time.Now().UnixNano(), 10)
		default:
			keyID = strconv.FormatInt(time.Now().UnixNano(), 10)
		}

		// Создаем ключ
		var key ds.Key
		if prefix == "" || prefix == "/" {
			key = ds.NewKey(keyID)
		} else {
			key = ds.NewKey(prefix).ChildString(keyID)
		}

		// Добавляем в batch
		err := batch.Put(ctx, key, []byte(line))
		if err != nil {
			errors++
			continue
		}

		batchCount++
		processed++

		// Коммитим batch если достигли размера
		if batchCount >= batchSize {
			if err := batch.Commit(ctx); err != nil {
				return processed, errors, fmt.Errorf("ошибка коммита batch: %w", err)
			}

			// Создаем новый batch
			batch, err = app.ds.Batch(ctx)
			if err != nil {
				return processed, errors, fmt.Errorf("ошибка создания нового batch: %w", err)
			}

			batchCount = 0

			// Показываем прогресс
			if processed%10000 == 0 {
				fmt.Printf("📈 Обработано: %d записей\n", processed)
			}
		}

		// Проверяем контекст
		select {
		case <-ctx.Done():
			return processed, errors, ctx.Err()
		default:
		}
	}

	// Коммитим оставшиеся данные
	if batchCount > 0 {
		if err := batch.Commit(ctx); err != nil {
			return processed, errors, fmt.Errorf("ошибка финального коммита: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return processed, errors, fmt.Errorf("ошибка чтения файла: %w", err)
	}

	return processed, errors, nil
}

func init() {
	commands = append(commands, &cli.Command{
		Name:    "load-jsonl",
		Aliases: []string{"import", "load"},
		Usage:   "Загрузить JSON Lines файлы в датастор",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "prefix",
				Aliases: []string{"p"},
				Value:   "/jsonl",
				Usage:   "Префикс для ключей",
			},
			&cli.StringFlag{
				Name:    "id-type",
				Aliases: []string{"t"},
				Value:   "unix-nano",
				Usage:   "Тип генерации ID: 'unix-nano' или 'tid'",
			},
			&cli.UintFlag{
				Name:    "clock-id",
				Aliases: []string{"c"},
				Value:   1,
				Usage:   "Clock ID для TID генератора (только для --id-type=tid)",
			},
			&cli.IntFlag{
				Name:    "batch-size",
				Aliases: []string{"b"},
				Value:   1000,
				Usage:   "Размер batch для записи в датастор",
			},
		},
		Action:    loadJSONL,
		ArgsUsage: "<путь к файлу/архиву/директории>",
		Description: `Загружает JSON Lines файлы в датастор.

Поддерживаемые источники:
- Отдельные файлы (.jsonl, .ndjson, .json)
- Сжатые файлы (.gz)
- ZIP архивы (.zip) 
- TAR архивы (.tar, .tar.gz, .tgz)
- Директории (обрабатывает все .jsonl файлы)

Каждая строка JSON становится отдельной записью в датасторе.
ID для ключей генерируется автоматически:
- unix-nano: timestamp в наносекундах  
- tid: TID из библиотеки tid (временно-упорядоченные ID)

Примеры:
  ues-ds load-jsonl data.jsonl
  ues-ds load-jsonl --prefix="/logs" --id-type=tid logs.zip
  ues-ds load-jsonl --batch-size=5000 /path/to/data/
  ues-ds import data.tar.gz -p "/events" -t tid -c 42`,
	})
}
