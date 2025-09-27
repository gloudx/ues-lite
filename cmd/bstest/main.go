package main

import (
	"context"
	"io"
	"log"
	"os"
	"ues-lite/blockstore"
	"ues-lite/datastore"

	badger4 "github.com/ipfs/go-ds-badger4"
)

func main() {

	opts := &badger4.DefaultOptions

	ds, err := datastore.NewDatastorage("./.data", opts)
	if err != nil {
		log.Fatal("Ошибка создания datastore:", err)
	}
	defer ds.Close()

	bs := blockstore.NewBlockstore(ds, "files/")

	c, err := bs.AddFile(context.Background(), "files/xxx.mp4")
	if err != nil {
		log.Fatal("Ошибка добавления файла:", err)
	}

	// c, err := bs.AddFile(context.Background(), f, false)
	// if err != nil {
	// 	log.Fatal("Ошибка добавления файла:", err)
	// }

	log.Println("CID добавленного файла:", c.String())

	r, err := bs.GetReader(context.Background(), c)
	if err != nil {
		log.Fatal("Ошибка получения файла по CID:", err)
	}
	defer r.Close()

	outFile, err := os.Create("output.mp4")
	if err != nil {
		log.Fatal("Ошибка создания выходного файла:", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, r)
	if err != nil {
		log.Fatal("Ошибка записи данных в выходной файл:", err)
	}

	log.Println("Файл успешно сохранен как output.mp4")

	f, err := bs.ListAll(context.Background())
	if err != nil {
		log.Fatal("Ошибка получения списка файлов:", err)
	}

	for {
		item := f(context.Background())
		if item == nil {
			break
		}
		log.Println(item)
	}
}
