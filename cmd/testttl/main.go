package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"ues-lite/datastore" // замените на ваш путь к пакету

	sd "github.com/ipfs/go-datastore"
	badger4 "github.com/ipfs/go-ds-badger4"
)

func main() {

	opts := &badger4.DefaultOptions

	ds, err := datastore.NewDatastorage("./.data", opts)
	if err != nil {
		log.Fatal("Ошибка создания datastore:", err)
	}
	defer ds.Close()

	ttlConfig := &datastore.TTLMonitorConfig{
		CheckInterval: 1 * time.Second, // проверяем каждые 5 секунд
		Enabled:       true,
		BufferSize:    100,
	}

	err = ds.EnableTTLMonitoring(ttlConfig)
	if err != nil {
		log.Fatal("Ошибка включения TTL мониторинга:", err)
	}

	fmt.Println("TTL мониторинг включен")

	ttlScript := `
		if (event.type === "ttl_expired") {
			console.log("TTL истек для ключа:", event.key);
			console.log("Последнее значение:", event.value);
			console.log("Время истечения:", event.metadata.expired_at);
		} else if (event.type === "put") {
			console.log("Новое значение:", event.key, "=", event.value);
		}
	`

	err = ds.CreateFilteredJSSubscription(
		context.Background(),
		"ttl-monitor",
		ttlScript,
		datastore.EventTTLExpired, // слушаем только TTL события
		datastore.EventPut,        // и события добавления
	)
	if err != nil {
		log.Fatal("Ошибка создания JS подписки:", err)
	}

	fmt.Println("JS подписчик создан")

	// Создаем канальный подписчик для примера
	channelSub := ds.SubscribeChannel("channel-ttl", 10)
	defer channelSub.Close()

	// Горутина для обработки событий из канала
	go func() {
		for event := range channelSub.Events() {
			if event.Type == datastore.EventTTLExpired {
				fmt.Printf("CHANNEL: TTL истек для ключа %s (последнее значение: %s)\n",
					event.Key.String(), string(event.Value))
			}
		}
	}()

	ctx := context.Background()

	// Тестируем TTL функциональность
	fmt.Println("\n=== Тест TTL функциональности ===")

	// Добавляем ключи с коротким TTL
	testKeys := []struct {
		key   string
		value string
		ttl   time.Duration
	}{
		{"test/short", "короткий TTL", 3 * time.Second},
		{"test/medium", "средний TTL", 8 * time.Second},
		{"test/long", "длинный TTL", 15 * time.Second},
	}

	for _, tk := range testKeys {
		key := sd.NewKey(tk.key)
		err = ds.PutWithTTL(ctx, key, []byte(tk.value), tk.ttl)
		if err != nil {
			log.Printf("Ошибка установки TTL для %s: %v", tk.key, err)
			continue
		}
		fmt.Printf("Установлен TTL %v для ключа %s\n", tk.ttl, tk.key)
	}

	// Добавляем обычный ключ без TTL для сравнения
	normalKey := sd.NewKey("test/normal")
	err = ds.Put(ctx, normalKey, []byte("обычный ключ без TTL"))
	if err != nil {
		log.Printf("Ошибка добавления обычного ключа: %v", err)
	} else {
		fmt.Println("Добавлен обычный ключ без TTL")
	}

	fmt.Println("\nОжидаем истечения TTL ключей...")
	fmt.Println("(Первый ключ должен истечь через ~3 секунды)")

	/**
		// Ждем чтобы увидеть события истечения TTL
		time.Sleep(20 * time.Second)

		// Проверяем какие ключи остались
		fmt.Println("\n=== Проверка оставшихся ключей ===")
		keysCh, errCh, err := ds.Keys(ctx, sd.NewKey("test/"))
		if err != nil {
			log.Fatal("Ошибка получения ключей:", err)
		}

		remainingKeys := []string{}
		for {
			select {
			case err, ok := <-errCh:
				if ok && err != nil {
					log.Printf("Ошибка: %v", err)
				}
			case key, ok := <-keysCh:
				if !ok {
					goto done
				}
				remainingKeys = append(remainingKeys, key.String())
			}
		}

	done:
		if len(remainingKeys) > 0 {
			fmt.Println("Оставшиеся ключи:")
			for _, key := range remainingKeys {
				fmt.Printf("  - %s\n", key)
			}
		} else {
			fmt.Println("Все ключи были удалены по TTL")
		}

		// Показываем текущую конфигурацию TTL мониторинга
		// config := ds.GetTTLMonitorConfig()
		// if config != nil {
		// 	fmt.Printf("\nТекущая конфигурация TTL мониторинга:\n")
		// 	fmt.Printf("  - Включен: %v\n", config.Enabled)
		// 	fmt.Printf("  - Интервал проверки: %v\n", config.CheckInterval)
		// 	fmt.Printf("  - Размер буфера: %d\n", config.BufferSize)
		// }
	*/

	select {}

	fmt.Println("\n=== Тест завершен ===")
}

// Функция для создания простого JS подписчика только для TTL событий
func createTTLOnlySubscriber(ds datastore.Datastore) error {
	script := `
		if (event.type === "ttl_expired") {
			console.log("🔔 TTL СОБЫТИЕ:");
			console.log("   Ключ:", event.key);
			console.log("   Время события:", event.timestamp);
			console.log("   Последнее значение:", event.value);
			
			if (event.metadata && event.metadata.expired_at) {
				console.log("   Истек в:", event.metadata.expired_at);
			}
			
			// Пример: отправка в webhook
			// HTTP.post("https://your-webhook.com/ttl-expired", {
			//     key: event.key,
			//     expired_at: event.metadata.expired_at,
			//     last_value: event.value
			// });
		}
	`

	return ds.CreateFilteredJSSubscription(
		context.Background(),
		"ttl-only-subscriber",
		script,
		datastore.EventTTLExpired,
	)
}

// Функция для тестирования массового истечения TTL
func testBatchTTLExpiration(ds datastore.Datastore) error {
	ctx := context.Background()

	fmt.Println("Создаем 10 ключей с TTL 2 секунды...")

	for i := 0; i < 10; i++ {
		key := sd.NewKey(fmt.Sprintf("batch/key-%d", i))
		value := fmt.Sprintf("batch value %d", i)

		err := ds.PutWithTTL(ctx, key, []byte(value), 2*time.Second)
		if err != nil {
			return fmt.Errorf("ошибка создания ключа %d: %w", i, err)
		}
	}

	fmt.Println("Ожидаем массового истечения TTL...")
	time.Sleep(5 * time.Second)

	return nil
}
