package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"ues-lite/datastore" // Adjust the import path as necessary

	ds "github.com/ipfs/go-datastore"
	badger4 "github.com/ipfs/go-ds-badger4"
)

func main() {

	opts := &badger4.DefaultOptions

	datas, err := datastore.NewDatastorage("./.data", opts)
	if err != nil {
		log.Fatal("Ошибка создания datastore:", err)
	}
	defer datas.Close()

	// Load sample data from JSON file
	data, err := ioutil.ReadFile("testdata/sample.json")
	if err != nil {
		log.Fatalf("Error reading sample data: %v", err)
	}

	var sampleData []map[string]interface{}
	if err := json.Unmarshal(data, &sampleData); err != nil {
		log.Fatalf("Error unmarshalling sample data: %v", err)
	}

	// Insert sample data into the datastore
	for _, i := range sampleData {
		key := i["key"].(string)
		value, err := json.Marshal(i["value"])
		if err != nil {
			log.Fatalf("Error marshalling data for key %s: %v", key, err)
		}
		if err := datas.Put(context.Background(), ds.NewKey("xxx").ChildString(key), value); err != nil {
			log.Fatalf("Error putting data into datastore: %v", err)
		}
	}

	d, _, err := datas.Iterator(context.Background(), ds.NewKey("xxx"), false)
	if err != nil {
		log.Fatalf("Error creating iterator: %v", err)
	}
	for k := range d {
		fmt.Println("Key:", k.Key.String(), string(k.Value))

	}

	// Define transformation parameters
	prefix := ds.NewKey("xxx") // Adjust the prefix as necessary
	extract := `{"xxx":name}`  // Define extract if needed
	patchs := []string{"iii=int#555"}       // Define patches if needed
	jqTransform := ""          // Define jqTransform if needed

	// Call the Transform function
	if err := datas.Transform(context.Background(), prefix, extract, patchs, jqTransform); err != nil {
		log.Fatalf("Error transforming data: %v", err)
	}

	fmt.Println("Data transformation completed successfully.")

	d, _, err = datas.Iterator(context.Background(), ds.NewKey("xxx"), false)
	if err != nil {
		log.Fatalf("Error creating iterator: %v", err)
	}
	for k := range d {
		fmt.Println("Key:", k.Key.String(), string(k.Value))

	}

}
