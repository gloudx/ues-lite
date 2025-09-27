package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"ues-lite/datastore" // замените на ваш путь к пакету

	dst "github.com/ipfs/go-datastore"
	badger4 "github.com/ipfs/go-ds-badger4"
)

const developersData = `{"id": 1, "name": "Алексей", "language": "Go", "experience": 5, "salary": 150000, "projects": ["microservice-auth", "api-gateway"], "skills": ["Docker", "Kubernetes", "PostgreSQL"], "team": "Backend", "active": true}
{"id": 2, "name": "Мария", "language": "JavaScript", "experience": 3, "salary": 120000, "projects": ["frontend-dashboard", "mobile-app"], "skills": ["React", "Node.js", "MongoDB"], "team": "Frontend", "active": true}
{"id": 3, "name": "Дмитрий", "language": "Python", "experience": 7, "salary": 180000, "projects": ["ml-pipeline", "data-analytics"], "skills": ["TensorFlow", "Pandas", "Redis"], "team": "Data Science", "active": false}
{"id": 4, "name": "Елена", "language": "Go", "experience": 4, "salary": 140000, "projects": ["payment-service", "notification-system"], "skills": ["gRPC", "RabbitMQ", "MySQL"], "team": "Backend", "active": true}
{"id": 5, "name": "Сергей", "language": "Java", "experience": 8, "salary": 200000, "projects": ["legacy-system", "migration-tool"], "skills": ["Spring", "Hibernate", "Oracle"], "team": "Backend", "active": true}`

func main() {

	opts := &badger4.DefaultOptions

	ds, err := datastore.NewDatastorage("./.data", opts)
	if err != nil {
		log.Fatal("Ошибка создания datastore:", err)
	}
	defer ds.Close()

	ctx := context.Background()

	for i, data := range strings.Split(developersData, "\n") {
		key := dst.NewKey(fmt.Sprintf("developers/%d", i+1))
		if err = ds.Put(ctx, key, []byte(data)); err != nil {
			log.Fatalf("Ошибка записи данных %s: %v", key, err)
		}
	}

	// res, err := ds.QueryJQ(ctx, "reduce inputs as $dev ({sum: 0, count: 0}; .sum += $dev.salary | .count += 1) | {A: .sum, B: .count, C:  .sum / .count}", &datastore.JQQueryOptions{
	// 	Prefix: dst.NewKey("developers/"),
	// })

	res, err := ds.QueryJQ(ctx, "reduce inputs as $dev ([]; . + [{x:$dev.name}])", &datastore.JQQueryOptions{
		Prefix: dst.NewKey("developers/"),
	})

	if err != nil {
		log.Fatalf("Ошибка выполнения jq запроса: %v", err)
	}

	fmt.Printf("Общая сумма зарплат: %v\n", res)

}
