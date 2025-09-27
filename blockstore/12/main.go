package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	chunker "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/filestore"
	posinfo "github.com/ipfs/boxo/filestore/posinfo"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	uio "github.com/ipfs/boxo/ipld/unixfs/io"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	query "github.com/ipfs/go-datastore/query"
	leveldb "github.com/ipfs/go-ds-leveldb"
	ipld "github.com/ipfs/go-ipld-format"
)

// SimpleIPFSNode представляет нашу простую IPFS ноду
type SimpleIPFSNode struct {
	// Datastore хранит метаданные и индексы
	datastore datastore.Datastore
	// Blockstore хранит сами блоки данных
	blockstore blockstore.Blockstore
	// BlockService предоставляет интерфейс для работы с блоками
	blockService blockservice.BlockService
	// DAGService позволяет работать с направленным ациклическим графом
	dagService ipld.DAGService

	fs *filestore.Filestore // Файловый менеджер для работы с файлами
}

// NewSimpleIPFSNode создает новую простую IPFS ноду
func NewSimpleIPFSNode(repoPath string) (*SimpleIPFSNode, error) {

	// Создаем директорию для хранения данных, если она не существует
	err := os.MkdirAll(repoPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать директорию репозитория: %v", err)
	}

	// Создаем путь для datastore в файловой системе
	datastorePath := filepath.Join(repoPath, "datastore")
	err = os.MkdirAll(datastorePath, 0755)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать директорию datastore: %v", err)
	}

	// Инициализируем filesystem datastore вместо LevelDB
	// Это будет хранить данные как обычные файлы в директории
	// ds, err := flatfs.CreateOrOpen(datastorePath, flatfs.IPFS_DEF_SHARD, false)
	// if err != nil {
	// 	return nil, fmt.Errorf("не удалось создать filesystem datastore: %v", err)
	// }

	// Инициализируем LevelDB datastore для хранения метаданных
	ds, err := leveldb.NewDatastore(filepath.Join(repoPath, "datastore"), nil)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать datastore: %v", err)
	}

	// iterate over goleveldb all keys
	// iter := ds.DB.NewIterator(util.BytesPrefix([]byte{}), nil)
	// for iter.Next() {
	// 	key := iter.Key()
	// 	value := iter.Value()
	// 	fmt.Printf("Key: %s, Value: %s\n", key, value[0:10]) // выводим первые 10 байт значения
	// }
	// iter.Release()

	// Создаем blockstore поверх datastore
	// Blockstore отвечает за хранение и получение блоков данных
	bs := blockstore.NewBlockstore(ds)
	

	fm := filestore.NewFileManager(ds, filepath.Join(repoPath, "fs2"), func(fm *filestore.FileManager) {
		fm.AllowFiles = true
	})

	//fs := filestore.NewFilestore(bs, fm)

	// Создаем offline exchange - это означает, что мы не подключаемся к сети
	// Для простоты мы работаем только локально
	exchange := offline.Exchange(bs)

	// BlockService объединяет blockstore и exchange
	blockService := blockservice.New(bs, exchange)

	// DAGService позволяет работать с IPLD графом
	dagService := merkledag.NewDAGService(blockService)

	return &SimpleIPFSNode{
		datastore:    ds,
		blockstore:   bs,
		blockService: blockService,
		dagService:   dagService,
		fs:           fs,
	}, nil
}

// AddFile добавляет файл в IPFS и возвращает его CID
func (node *SimpleIPFSNode) AddFile(filePath string) (cid.Cid, error) {
	// Открываем файл для чтения
	file, err := os.Open(filePath)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось открыть файл: %v", err)
	}
	defer file.Close()

	// Получаем информацию о файле
	stat, err := file.Stat()
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось получить информацию о файле: %v", err)
	}

	// Создаем файловый узел для IPFS
	f := files.NewReaderFile(file)

	// Настраиваем параметры импорта
	// Chunker разбивает файл на блоки определенного размера
	chunker := chunker.NewSizeSplitter(f, chunker.DefaultBlockSize)

	// Balanced layout создает сбалансированное дерево блоков
	params := helpers.DagBuilderParams{
		Maxlinks:   helpers.DefaultLinksPerBlock, // Максимальное количество ссылок на блок
		RawLeaves:  true,                         // Использовать raw блоки для листьев
		CidBuilder: nil,                          // Использовать CID builder по умолчанию
		Dagserv:    node.dagService,
	}

	// Создаем DAG builder
	db, err := params.New(chunker)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось создать DAG builder: %v", err)
	}

	// Импортируем файл в IPFS
	nd, err := balanced.Layout(db)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось импортировать файл: %v", err)
	}

	fmt.Printf("Файл '%s' (размер: %d байт) успешно добавлен в IPFS\n",
		filepath.Base(filePath), stat.Size())

	return nd.Cid(), nil
}

// AddString добавляет строку в IPFS и возвращает её CID
func (node *SimpleIPFSNode) AddString(content string) (cid.Cid, error) {
	// Создаем reader из строки
	reader := strings.NewReader(content)
	f := files.NewReaderFile(reader)

	// Используем тот же процесс, что и для файлов
	chunker := chunker.NewSizeSplitter(f, chunker.DefaultBlockSize)

	params := helpers.DagBuilderParams{
		Maxlinks:   helpers.DefaultLinksPerBlock,
		RawLeaves:  true,
		CidBuilder: nil,
		Dagserv:    node.dagService,
	}

	db, err := params.New(chunker)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось создать DAG builder: %v", err)
	}

	nd, err := balanced.Layout(db)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось импортировать строку: %v", err)
	}

	fmt.Printf("Строка (длина: %d символов) успешно добавлена в IPFS\n", len(content))
	return nd.Cid(), nil
}

func (node *SimpleIPFSNode) get(cidStr string) (ipld.Node, error) {
	// Преобразуем строку CID в объект cid.Cid
	c, err := cid.Decode(cidStr)
	if err != nil {
		return nil, fmt.Errorf("не удалось декодировать CID: %v", err)
	}

	// Получаем узел по CID
	nd, err := node.dagService.Get(context.Background(), c)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить узел: %v", err)
	}

	return nd, nil
}

// GetFile получает файл из IPFS по его CID
func (node *SimpleIPFSNode) GetFile(c cid.Cid, outputPath string) error {
	// Получаем узел по CID
	nd, err := node.dagService.Get(context.Background(), c)
	if err != nil {
		return fmt.Errorf("не удалось получить узел: %v", err)
	}

	// Создаем UnixFS reader для чтения данных
	dr, err := uio.NewDagReader(context.Background(), nd, node.dagService)
	if err != nil {
		return fmt.Errorf("не удалось создать DAG reader: %v", err)
	}
	defer dr.Close()

	// Создаем выходной файл
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("не удалось создать выходной файл: %v", err)
	}
	defer outFile.Close()

	// Копируем данные из IPFS в файл
	_, err = io.Copy(outFile, dr)
	if err != nil {
		return fmt.Errorf("не удалось записать данные в файл: %v", err)
	}

	fmt.Printf("Файл успешно получен из IPFS и сохранен как '%s'\n", outputPath)
	return nil
}

// GetString получает строку из IPFS по её CID
func (node *SimpleIPFSNode) GetString(c cid.Cid) (string, error) {
	// Получаем узел по CID
	nd, err := node.dagService.Get(context.Background(), c)
	if err != nil {
		return "", fmt.Errorf("не удалось получить узел: %v", err)
	}

	// Создаем DAG reader
	dr, err := uio.NewDagReader(context.Background(), nd, node.dagService)
	if err != nil {
		return "", fmt.Errorf("не удалось создать DAG reader: %v", err)
	}
	defer dr.Close()

	// Читаем все данные в память
	data, err := io.ReadAll(dr)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать данные: %v", err)
	}

	return string(data), nil
}

// Close корректно закрывает ноду и освобождает ресурсы
func (node *SimpleIPFSNode) Close() error {
	return node.datastore.Close()
}

// Главная функция демонстрирует использование нашей простой IPFS ноды
func main() {
	fmt.Println("=== Инициализация простой IPFS ноды ===")
	// Создаем новую ноду в директории ./ipfs-repo
	node, err := NewSimpleIPFSNode("./ipfs-repo")
	if err != nil {
		fmt.Printf("Ошибка при создании ноды: %v\n", err)
		return
	}
	defer node.Close()
	fmt.Println("IPFS нода успешно инициализирована!")

	// node.blockstore.Put(context.Background(), blocks.NewBlock([]byte("Hello IPFS!"))) // Пример добавления блока

	res, err := node.datastore.Query(context.Background(), query.Query{}) // Пример запроса всех ключей
	if err != nil {
		log.Fatal("Ошибка при запросе ключей из datastore:", err)
	}
	defer res.Close()

	fmt.Println("Ключи в datastore:")
	for {
		e, ok := res.NextSync()
		if !ok {
			break
		}
		if e.Error != nil {
			log.Printf("Ошибка при получении ключа: %s", e.Error)
			continue
		}
		fmt.Printf("Ключ: %s == %s\n", e.Key, string(e.Value))
	}

	items, err := node.blockstore.AllKeysChan(context.Background()) // Пример получения всех ключей
	if err != nil {
		log.Fatal("Ошибка при получении ключей из блокстора:", err)
	}
	for item := range items {
		fmt.Printf("Ключ в блоксторе: %s\n", string(item.Bytes()))
	}

	panic("Проверка работы IPFS ноды")

	file, err := os.Open(filepath.Join("./ipfs-repo", "fs2", "go.mod"))
	if err != nil {
		log.Fatal("Ошибка при открытии файла:", err)
	}
	defer file.Close()

	if err := node.blockstore.Put(context.Background(), blocks.NewBlock([]byte("Hello IPFS!"))); err != nil {
		log.Fatal("Ошибка при добавлении блока в блокстор:", err)
	}

	// panic("Проверка работы IPFS ноды")

	data, err := io.ReadAll(file)
	if err != nil {
		log.Fatal("Ошибка при чтении файла:", err)
	}

	n := &posinfo.FilestoreNode{
		PosInfo: &posinfo.PosInfo{
			FullPath: filepath.Join("./ipfs-repo", "fs2", "go.mod"),
			Offset:   0,
		},
		Node: merkledag.NewRawNode(data),
	}

	if err := node.fs.Put(context.Background(), n); err != nil {
		log.Fatal("Ошибка при добавлении файла в Filestore:", err)
	}

	//QmYmQTLi7x8Kx1PivBUsEtdtiTJxUA3EdotVcXH9VoBBYU

	// Демонстрация 1: Добавление строки
	// fmt.Println("\n=== Добавление строки в IPFS ===")
	// testString := "Привет, IPFS! Это тестовая строка для демонстрации."
	// stringCid, err := node.AddString(testString)
	// if err != nil {
	// 	fmt.Printf("Ошибка при добавлении строки: %v\n", err)
	// 	return
	// }
	// fmt.Printf("CID строки: %s\n", stringCid.String())

	// Демонстрация 2: Получение строки обратно
	// fmt.Println("\n=== Получение строки из IPFS ===")
	// retrievedString, err := node.GetString(stringCid)
	// if err != nil {
	// 	fmt.Printf("Ошибка при получении строки: %v\n", err)
	// 	return
	// }
	// fmt.Printf("Полученная строка: %s\n", retrievedString)

	// Проверяем, что данные совпадают
	// if testString == retrievedString {
	// 	fmt.Println("✅ Успех! Исходная и полученная строки совпадают")
	// } else {
	// 	fmt.Println("❌ Ошибка! Строки не совпадают")
	// }

	// Демонстрация 3: Работа с файлами (если создать тестовый файл)
	fmt.Println("\n=== Пример работы с файлами ===")

	// Создаем тестовый файл
	testFile := "x.png"
	// testContent := "Это тестовый файл для демонстрации IPFS.\nОн содержит несколько строк текста.\nIPFS позволяет хранить файлы децентрализованно!"
	// err = os.WriteFile(testFile, []byte(testContent), 0644)
	// if err != nil {
	// 	fmt.Printf("Не удалось создать тестовый файл: %v\n", err)
	// 	return
	// }
	// defer os.Remove(testFile) // Удаляем тестовый файл после завершения

	// Добавляем файл в IPFS
	fileCid, err := node.AddFile(testFile)
	if err != nil {
		fmt.Printf("Ошибка при добавлении файла: %v\n", err)
		return
	}

	fmt.Printf("CID файла: %s\n", fileCid.String())

	// Получаем файл обратно
	outputFile := "retrieved_test.png"
	err = node.GetFile(fileCid, outputFile)
	if err != nil {
		fmt.Printf("Ошибка при получении файла: %v\n", err)
		return
	}
	// defer os.Remove(outputFile) // Удаляем полученный файл

	// Проверяем содержимое
	// retrievedContent, err := os.ReadFile(outputFile)
	// if err != nil {
	// 	fmt.Printf("Не удалось прочитать полученный файл: %v\n", err)
	// 	return
	// }

	// if testContent == string(retrievedContent) {
	// 	fmt.Println("✅ Успех! Исходный и полученный файлы совпадают")
	// } else {
	// 	fmt.Println("❌ Ошибка! Файлы не совпадают")
	// }

	// fmt.Println("\n=== Демонстрация завершена ===")
	// fmt.Println("Простая IPFS нода успешно продемонстрировала основные функции:")
	// fmt.Println("- Добавление и получение строк")
	// fmt.Println("- Добавление и получение файлов")
	// fmt.Println("- Работа с Content Identifiers (CID)")
}
