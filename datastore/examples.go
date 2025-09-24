package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	ds "github.com/ipfs/go-datastore"
)

// Примеры использования Views

// ExampleCreateUserProfilesView демонстрирует создание view для профилей пользователей
func ExampleCreateUserProfilesView() {
	// Предполагаем, что у нас есть datastore
	var store Datastore // = NewDatastorage(...)

	ctx := context.Background()

	// Создаем view для активных пользователей
	activeUsersConfig := ViewConfig{
		ID:           "active_users",
		Name:         "Active Users",
		Description:  "View of all active user profiles",
		SourcePrefix: "/users/",
		TargetPrefix: "/views/active_users/",

		// Фильтруем только активных пользователей
		FilterScript: `
			// Проверяем, что это JSON объект
			if (data.json && typeof data.json === 'object') {
				// Фильтруем активных пользователей
				return data.json.active === true && data.json.lastLogin;
			}
			return false;
		`,

		// Трансформируем данные для view
		TransformScript: `
			var user = data.json;
			return {
				id: user.id,
				name: user.name,
				email: user.email,
				lastLogin: user.lastLogin,
				score: user.score || 0
			};
		`,

		// Сортируем по последнему входу
		SortScript: `
			var user = data.json;
			if (user.lastLogin) {
				// Возвращаем timestamp для сортировки (более свежие = выше)
				return new Date(user.lastLogin).getTime();
			}
			return 0;
		`,

		EnableCaching:   true,
		CacheTTL:        5 * time.Minute,
		AutoRefresh:     true,
		RefreshDebounce: 10 * time.Second,
		MaxResults:      100,
	}

	view, err := store.CreateView(ctx, activeUsersConfig)
	if err != nil {
		log.Printf("Failed to create view: %v", err)
		return
	}

	// Выполняем view
	results, err := view.Execute(ctx)
	if err != nil {
		log.Printf("Failed to execute view: %v", err)
		return
	}

	fmt.Printf("Found %d active users\n", len(results))
	for _, result := range results {
		fmt.Printf("User: %v\n", result.Value)
	}
}

// ExampleCreateOrdersView демонстрирует создание view для заказов
func ExampleCreateOrdersView() {
	var store Datastore // = NewDatastorage(...)

	ctx := context.Background()

	// Создаем view для заказов за последние 30 дней
	recentOrdersConfig := ViewConfig{
		ID:           "recent_orders",
		Name:         "Recent Orders",
		Description:  "Orders from the last 30 days",
		SourcePrefix: "/orders/",
		TargetPrefix: "/views/recent_orders/",

		FilterScript: `
			if (data.json && data.json.createdAt) {
				var orderDate = new Date(data.json.createdAt);
				var thirtyDaysAgo = new Date();
				thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
				
				return orderDate >= thirtyDaysAgo && data.json.status !== 'cancelled';
			}
			return false;
		`,

		TransformScript: `
			var order = data.json;
			return {
				id: order.id,
				customerId: order.customerId,
				total: order.total,
				status: order.status,
				createdAt: order.createdAt,
				daysSinceOrder: Math.floor((Date.now() - new Date(order.createdAt)) / (1000 * 60 * 60 * 24))
			};
		`,

		SortScript: `
			return data.json.total || 0; // Сортируем по сумме заказа
		`,

		EnableCaching:   true,
		CacheTTL:        1 * time.Hour,
		AutoRefresh:     true,
		RefreshDebounce: 30 * time.Second,
		MaxResults:      1000,
	}

	_, err := store.CreateView(ctx, recentOrdersConfig)
	if err != nil {
		log.Printf("Failed to create orders view: %v", err)
		return
	}
}

// ExampleCreateTopProductsView создает view для топ-продуктов
func ExampleCreateTopProductsView() {
	var store Datastore // = NewDatastorage(...)

	ctx := context.Background()

	topProductsConfig := ViewConfig{
		ID:           "top_products",
		Name:         "Top Products",
		Description:  "Most popular products by sales",
		SourcePrefix: "/products/",
		TargetPrefix: "/views/top_products/",

		FilterScript: `
			return data.json && 
				   data.json.active === true && 
				   data.json.salesCount > 0;
		`,

		TransformScript: `
			var product = data.json;
			return {
				id: product.id,
				name: product.name,
				category: product.category,
				price: product.price,
				salesCount: product.salesCount,
				rating: product.averageRating || 0,
				revenue: (product.price || 0) * (product.salesCount || 0)
			};
		`,

		SortScript: `
			var product = data.json;
			// Комбинированный score: продажи * рейтинг
			return (product.salesCount || 0) * (product.averageRating || 1);
		`,

		EnableCaching:   true,
		CacheTTL:        2 * time.Hour,
		AutoRefresh:     true,
		RefreshDebounce: 5 * time.Minute,
		MaxResults:      50,
	}

	_, err := store.CreateView(ctx, topProductsConfig)
	if err != nil {
		log.Printf("Failed to create top products view: %v", err)
		return
	}
}

// ExampleUsingViews демонстрирует работу с созданными view
func ExampleUsingViews() {
	var store Datastore // = NewDatastorage(...)

	ctx := context.Background()

	// Добавляем тестовые данные
	testUsers := []struct {
		ID   string
		Data map[string]interface{}
	}{
		{"user1", map[string]interface{}{
			"id": "user1", "name": "Alice", "email": "alice@example.com",
			"active": true, "lastLogin": "2025-09-20T10:00:00Z", "score": 95,
		}},
		{"user2", map[string]interface{}{
			"id": "user2", "name": "Bob", "email": "bob@example.com",
			"active": false, "lastLogin": "2025-08-15T14:30:00Z", "score": 75,
		}},
		{"user3", map[string]interface{}{
			"id": "user3", "name": "Charlie", "email": "charlie@example.com",
			"active": true, "lastLogin": "2025-09-22T16:45:00Z", "score": 88,
		}},
	}

	// Сохраняем тестовые данные
	for _, user := range testUsers {
		data, _ := json.Marshal(user.Data)
		key := ds.NewKey("/users/").ChildString(user.ID)
		store.Put(ctx, key, data)
	}

	// Создаем простой view для активных пользователей
	_, err := store.CreateSimpleView(ctx, "simple_active_users", "Simple Active Users", "/users/",
		`data.json && data.json.active === true`)
	if err != nil {
		log.Printf("Failed to create simple view: %v", err)
		return
	}

	// Даем время на обработку событий
	time.Sleep(2 * time.Second)

	// Получаем результаты view
	results, err := store.ExecuteView(ctx, "simple_active_users")
	if err != nil {
		log.Printf("Failed to execute view: %v", err)
		return
	}

	fmt.Printf("Active users view returned %d results:\n", len(results))
	for _, result := range results {
		fmt.Printf("- %v\n", result.Value)
	}

	// Проверяем кешированные результаты
	cached, found, err := store.GetViewCached(ctx, "simple_active_users")
	if err != nil {
		log.Printf("Failed to get cached results: %v", err)
		return
	}

	if found {
		fmt.Printf("Found %d cached results\n", len(cached))
	} else {
		fmt.Println("No cached results found")
	}

	// Получаем статистику view
	if stats, found := store.GetViewStats("simple_active_users"); found {
		fmt.Printf("View stats: refreshes=%d, cache_hits=%d, cache_misses=%d\n",
			stats.RefreshCount, stats.CacheHits, stats.CacheMisses)
	}
}

// ExampleViewWithRanges демонстрирует использование view с диапазонами ключей
func ExampleViewWithRanges() {
	var store Datastore // = NewDatastorage(...)

	ctx := context.Background()

	// Создаем view с ограничением по диапазону ключей
	rangedConfig := ViewConfig{
		ID:           "users_a_to_m",
		Name:         "Users A-M",
		Description:  "Users with names starting from A to M",
		SourcePrefix: "/users/",
		StartKey:     "/users/a",
		EndKey:       "/users/n", // Не включается

		FilterScript: `
			return data.json && 
				   data.json.name && 
				   data.json.name.toLowerCase().charAt(0) <= 'm';
		`,

		TransformScript: `
			var user = data.json;
			return {
				name: user.name,
				firstLetter: user.name.charAt(0).toUpperCase()
			};
		`,

		SortScript: `
			return data.json.name ? -data.json.name.localeCompare('') : 0; // Алфавитная сортировка
		`,

		EnableCaching: true,
		AutoRefresh:   true,
		MaxResults:    100,
	}

	view, err := store.CreateView(ctx, rangedConfig)
	if err != nil {
		log.Printf("Failed to create ranged view: %v", err)
		return
	}

	// Выполняем view с дополнительным ограничением диапазона
	startKey := ds.NewKey("/users/b")
	endKey := ds.NewKey("/users/f")

	results, err := view.ExecuteWithRange(ctx, startKey, endKey)
	if err != nil {
		log.Printf("Failed to execute ranged view: %v", err)
		return
	}

	fmt.Printf("Users B-F: %d results\n", len(results))
	for _, result := range results {
		fmt.Printf("- %v\n", result.Value)
	}
}

// ExampleViewAutoRefresh демонстрирует автоматическое обновление view
func ExampleViewAutoRefresh() {
	var store Datastore // = NewDatastorage(...)

	ctx := context.Background()

	// Создаем view с автоматическим обновлением
	autoRefreshConfig := ViewConfig{
		ID:              "auto_refresh_users",
		Name:            "Auto Refresh Users",
		SourcePrefix:    "/users/",
		FilterScript:    `data.json && data.json.active === true`,
		EnableCaching:   true,
		AutoRefresh:     true,
		RefreshDebounce: 2 * time.Second,
	}

	view, err := store.CreateView(ctx, autoRefreshConfig)
	if err != nil {
		log.Printf("Failed to create auto-refresh view: %v", err)
		return
	}

	// Получаем начальные результаты
	initialResults, err := view.Execute(ctx)
	if err != nil {
		log.Printf("Failed to get initial results: %v", err)
		return
	}
	fmt.Printf("Initial results: %d\n", len(initialResults))

	// Добавляем нового пользователя - view должен автоматически обновиться
	newUser := map[string]interface{}{
		"id": "user4", "name": "David", "email": "david@example.com",
		"active": true, "lastLogin": "2025-09-23T12:00:00Z",
	}

	data, _ := json.Marshal(newUser)
	key := ds.NewKey("/users/user4")
	store.Put(ctx, key, data)

	// Ждем обновления (debounce + обработка)
	time.Sleep(5 * time.Second)

	// Проверяем обновленные результаты
	updatedResults, _, err := view.GetCached(ctx)
	if err != nil {
		log.Printf("Failed to get updated results: %v", err)
		return
	}

	if found := len(updatedResults) > 0; found {
		fmt.Printf("Updated results: %d (should be %d)\n", len(updatedResults), len(initialResults)+1)
	}
}
