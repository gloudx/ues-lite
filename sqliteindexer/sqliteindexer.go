package sqliteindexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteIndexer struct {
	db *sql.DB
	mu sync.RWMutex
}
type IndexMetadata struct {
	Collection string                 `json:"collection"`
	RKey       string                 `json:"rkey"`
	RecordType string                 `json:"record_type"`
	Data       map[string]interface{} `json:"data"`
	SearchText string                 `json:"search_text"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}
type SearchQuery struct {
	Collection    string                 `json:"collection,omitempty"`
	RecordType    string                 `json:"record_type,omitempty"`
	Filters       map[string]interface{} `json:"filters,omitempty"`
	FullTextQuery string                 `json:"full_text_query,omitempty"`
	SortBy        string                 `json:"sort_by,omitempty"`
	SortOrder     string                 `json:"sort_order,omitempty"`
	Limit         int                    `json:"limit,omitempty"`
	Offset        int                    `json:"offset,omitempty"`
}
type SearchResult struct {
	CID        cid.Cid                `json:"cid"`
	Collection string                 `json:"collection"`
	RKey       string                 `json:"rkey"`
	RecordType string                 `json:"record_type"`
	Data       map[string]interface{} `json:"data"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Relevance  float64                `json:"relevance,omitempty"`
}

func NewSQLiteIndexer(dbPath string) (*SQLiteIndexer, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}
	indexer := &SQLiteIndexer{
		db: db,
	}
	if err := indexer.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	return indexer, nil
}
func (idx *SQLiteIndexer) initSchema() error {
	schema := `
	-- ===============================================
	-- ОСНОВНАЯ ТАБЛИЦА ЗАПИСЕЙ
	-- ===============================================
	-- 
	-- НАЗНАЧЕНИЕ:
	-- Центральная таблица, хранящая метаданные всех записей в UES.
	-- Служит мостом между content-addressed storage (CID) и структурированными запросами.
	--
	-- ДИЗАЙН:
	-- - cid как PRIMARY KEY обеспечивает уникальность и быструю навигацию
	-- - collection + rkey образуют логический составной ключ
	-- - data хранит JSON сериализованные IPLD данные
	-- - search_text содержит агрегированный текст для FTS5
	--
	-- ИНДЕКСАЦИЯ:
	-- Таблица оптимизирована для частых запросов по коллекциям и типам записей
	CREATE TABLE IF NOT EXISTS records (
		cid TEXT PRIMARY KEY,              -- Content Identifier - связь с blockstore
		collection TEXT NOT NULL,          -- Логическая коллекция (posts, users, comments)
		rkey TEXT NOT NULL,                -- Уникальный ключ записи в коллекции
		record_type TEXT NOT NULL,         -- Тип записи для дополнительной категоризации
		data TEXT NOT NULL,                -- JSON сериализованные IPLD данные
		search_text TEXT,                  -- Агрегированный текст для полнотекстового поиска
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,  -- Время создания записи
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,  -- Время последнего обновления
		UNIQUE(collection, rkey)           -- Бизнес-ключ: уникальность в рамках коллекции
	);
	-- ===============================================
	-- ИНДЕКСЫ ДЛЯ ОПТИМИЗАЦИИ ЗАПРОСОВ
	-- ===============================================
	--
	-- СТРАТЕГИЯ ИНДЕКСАЦИИ:
	-- Индексы создаются на основе частых паттернов запросов:
	-- 1. Поиск по коллекции (самый частый)
	-- 2. Фильтрация по типу записи
	-- 3. Комбинированные запросы коллекция+тип
	-- 4. Сортировка по времени создания/обновления
	-- Индекс для запросов "все записи коллекции X"
	CREATE INDEX IF NOT EXISTS idx_records_collection ON records(collection);
	-- Индекс для фильтрации по типу записи
	CREATE INDEX IF NOT EXISTS idx_records_type ON records(record_type);
	-- Составной индекс для запросов "записи типа Y в коллекции X"
	CREATE INDEX IF NOT EXISTS idx_records_collection_type ON records(collection, record_type);
	-- Индексы для сортировки по времени (ORDER BY оптимизация)
	CREATE INDEX IF NOT EXISTS idx_records_created_at ON records(created_at);
	CREATE INDEX IF NOT EXISTS idx_records_updated_at ON records(updated_at);
	-- ===============================================
	-- FTS5 ПОЛНОТЕКСТОВЫЙ ПОИСК
	-- ===============================================
	--
	-- АРХИТЕКТУРА FTS5:
	-- records_fts - это виртуальная таблица SQLite FTS5, которая:
	-- - Создает инвертированный индекс для быстрого текстового поиска
	-- - Поддерживает ранжирование результатов по релевантности
	-- - Автоматически обрабатывает стемминг и токенизацию
	-- - Связана с основной таблицей через content_rowid
	--
	-- FIELDS:
	-- - cid: для связи с основной таблицей
	-- - collection: для фильтрации поиска по коллекциям
	-- - rkey: для идентификации записи
	-- - search_text: индексируемый текстовый контент
	--
	-- НАСТРОЙКИ:
	-- - content='records': FTS5 синхронизируется с таблицей records
	-- - content_rowid='rowid': использует SQLite rowid для связи
	CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
		cid,           -- Content Identifier для связи
		collection,    -- Коллекция для фильтрации FTS запросов
		rkey,          -- Ключ записи
		search_text,   -- Индексируемый текстовый контент
		content='records',        -- Связь с основной таблицей
		content_rowid='rowid'     -- Использование SQLite rowid
	);
	-- ===============================================
	-- ТРИГГЕРЫ ДЛЯ СИНХРОНИЗАЦИИ FTS5
	-- ===============================================
	--
	-- МЕХАНИЗМ СИНХРОНИЗАЦИИ:
	-- Триггеры обеспечивают автоматическую синхронизацию между
	-- основной таблицей records и FTS5 таблицей records_fts.
	-- Это гарантирует консистентность полнотекстового индекса.
	--
	-- СОБЫТИЯ:
	-- 1. INSERT: добавление новой записи в FTS индекс
	-- 2. DELETE: удаление записи из FTS индекса
	-- 3. UPDATE: пересоздание записи в FTS индексе
	-- Триггер вставки: добавляет новую запись в FTS5 при INSERT в records
	CREATE TRIGGER IF NOT EXISTS records_fts_insert AFTER INSERT ON records BEGIN
		INSERT INTO records_fts(cid, collection, rkey, search_text) 
		VALUES (new.cid, new.collection, new.rkey, new.search_text);
	END;
	-- Триггер удаления: удаляет запись из FTS5 при DELETE из records
	CREATE TRIGGER IF NOT EXISTS records_fts_delete AFTER DELETE ON records BEGIN
		DELETE FROM records_fts WHERE cid = old.cid;
	END;
	-- Триггер обновления: пересоздает запись в FTS5 при UPDATE records
	-- Использует DELETE + INSERT для корректного обновления FTS индекса
	CREATE TRIGGER IF NOT EXISTS records_fts_update AFTER UPDATE ON records BEGIN
		DELETE FROM records_fts WHERE cid = old.cid;
		INSERT INTO records_fts(cid, collection, rkey, search_text) 
		VALUES (new.cid, new.collection, new.rkey, new.search_text);
	END;
	-- ===============================================
	-- ТАБЛИЦА АТРИБУТОВ ДЛЯ СТРУКТУРИРОВАННОГО ПОИСКА
	-- ===============================================
	--
	-- НАЗНАЧЕНИЕ:
	-- record_attributes обеспечивает быстрые структурированные запросы
	-- по произвольным полям IPLD записей. Эта таблица реализует паттерн
	-- Entity-Attribute-Value (EAV) для гибкой индексации JSON данных.
	--
	-- АРХИТЕКТУРА EAV:
	-- - Каждый атрибут записи хранится как отдельная строка
	-- - Поддерживается типизация значений (string, number, boolean, datetime)
	-- - Быстрые индексы по имени атрибута и значению
	-- - Каскадное удаление при удалении основной записи
	--
	-- ПРИМЕНЕНИЕ:
	-- Позволяет делать запросы типа:
	-- "найти все посты пользователя X"
	-- "найти все записи с рейтингом > 5"
	-- "найти записи, созданные в 2024 году"
	CREATE TABLE IF NOT EXISTS record_attributes (
		cid TEXT NOT NULL,                 -- Связь с основной записью
		attribute_name TEXT NOT NULL,     -- Имя атрибута (например: "author", "rating", "tags")
		attribute_value TEXT NOT NULL,    -- Значение атрибута (всегда строка для универсальности)
		value_type TEXT NOT NULL,         -- Тип значения: 'string', 'number', 'boolean', 'datetime', 'json'
		PRIMARY KEY (cid, attribute_name), -- Композитный первичный ключ
		FOREIGN KEY (cid) REFERENCES records(cid) ON DELETE CASCADE  -- Каскадное удаление
	);
	-- ИНДЕКСЫ ДЛЯ БЫСТРЫХ ФИЛЬТРОВ:
	-- Индекс для запросов "WHERE attribute_name = X AND attribute_value = Y"
	CREATE INDEX IF NOT EXISTS idx_attr_name_value ON record_attributes(attribute_name, attribute_value);
	-- Индекс для типизированных запросов "WHERE attribute_name = X AND value_type = Y"
	CREATE INDEX IF NOT EXISTS idx_attr_name_type ON record_attributes(attribute_name, value_type);
	-- ===============================================
	-- ТРИГГЕР ДЛЯ АВТОМАТИЧЕСКОГО ОБНОВЛЕНИЯ ВРЕМЕННЫХ МЕТОК
	-- ===============================================
	--
	-- МЕХАНИЗМ:
	-- Триггер автоматически обновляет поле updated_at при любом изменении
	-- записи в таблице records. Это обеспечивает точное отслеживание
	-- времени последнего изменения без необходимости помнить об этом
	-- в прикладном коде.
	CREATE TRIGGER IF NOT EXISTS update_records_timestamp 
		AFTER UPDATE ON records
	BEGIN
		UPDATE records SET updated_at = CURRENT_TIMESTAMP WHERE cid = NEW.cid;
	END;
	-- ===============================================
	-- ПРЕДСТАВЛЕНИЕ ДЛЯ СТАТИСТИКИ КОЛЛЕКЦИЙ
	-- ===============================================
	--
	-- НАЗНАЧЕНИЕ:
	-- collection_stats - это материализованное представление (VIEW),
	-- которое предоставляет быстрый доступ к аналитической информации
	-- о коллекциях без необходимости выполнять тяжелые агрегирующие запросы.
	--
	-- МЕТРИКИ:
	-- - record_count: общее количество записей в коллекции
	-- - type_count: количество различных типов записей
	-- - first_record: время создания первой записи (для аудита)
	-- - last_updated: время последнего обновления любой записи
	--
	-- ПРОИЗВОДИТЕЛЬНОСТЬ:
	-- SQLite оптимизирует VIEW запросы, и для небольших коллекций
	-- агрегация выполняется быстро. Для больших коллекций можно
	-- рассмотреть материализованные таблицы с инкрементальным обновлением.
	CREATE VIEW IF NOT EXISTS collection_stats AS
	SELECT 
		collection,                        -- Имя коллекции
		COUNT(*) as record_count,          -- Общее количество записей
		COUNT(DISTINCT record_type) as type_count,  -- Количество типов записей
		MIN(created_at) as first_record,   -- Время создания первой записи
		MAX(updated_at) as last_updated    -- Время последнего обновления
	FROM records 
	GROUP BY collection;
	`
	_, err := idx.db.Exec(schema)
	return err
}
func (idx *SQLiteIndexer) IndexRecord(ctx context.Context, recordCID cid.Cid, metadata IndexMetadata) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	dataJSON, err := json.Marshal(metadata.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal record data: %w", err)
	}
	_, err = idx.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO records 
		(cid, collection, rkey, record_type, data, search_text, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, recordCID.String(), metadata.Collection, metadata.RKey, metadata.RecordType,
		string(dataJSON), metadata.SearchText, metadata.CreatedAt, metadata.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to index record: %w", err)
	}
	if err := idx.indexAttributes(ctx, recordCID.String(), metadata.Data); err != nil {
		return fmt.Errorf("failed to index attributes: %w", err)
	}
	return nil
}
func (idx *SQLiteIndexer) indexAttributes(ctx context.Context, cidStr string, data map[string]interface{}) error {
	_, err := idx.db.ExecContext(ctx, "DELETE FROM record_attributes WHERE cid = ?", cidStr)
	if err != nil {
		return err
	}
	for key, value := range data {
		valueStr, valueType := getAttributeValue(value)
		_, err = idx.db.ExecContext(ctx, `
			INSERT INTO record_attributes (cid, attribute_name, attribute_value, value_type)
			VALUES (?, ?, ?, ?)
		`, cidStr, key, valueStr, valueType)
		if err != nil {
			return err
		}
	}
	return nil
}
func getAttributeValue(value interface{}) (string, string) {
	switch v := value.(type) {
	case string:
		return v, "string"
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v), "number"
	case bool:
		return fmt.Sprintf("%t", v), "boolean"
	case time.Time:
		return v.Format(time.RFC3339), "datetime"
	default:
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes), "json"
		}
		return fmt.Sprintf("%v", v), "string"
	}
}
func (idx *SQLiteIndexer) DeleteRecord(ctx context.Context, recordCID cid.Cid) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	_, err := idx.db.ExecContext(ctx, "DELETE FROM records WHERE cid = ?", recordCID.String())
	return err
}
func (idx *SQLiteIndexer) SearchRecords(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var results []SearchResult
	var err error
	if query.FullTextQuery != "" {
		results, err = idx.searchFullText(ctx, query)
	} else {
		results, err = idx.searchStructured(ctx, query)
	}
	return results, err
}
func (idx *SQLiteIndexer) searchFullText(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	sql := `
		SELECT r.cid, r.collection, r.rkey, r.record_type, r.data, r.created_at, r.updated_at,
		       fts.rank as relevance
		FROM records_fts fts
		JOIN records r ON r.cid = fts.cid
		WHERE records_fts MATCH ?
	`
	args := []interface{}{query.FullTextQuery}
	if query.Collection != "" {
		sql += " AND r.collection = ?"
		args = append(args, query.Collection)
	}
	if query.RecordType != "" {
		sql += " AND r.record_type = ?"
		args = append(args, query.RecordType)
	}
	if query.SortBy != "" {
		order := "ASC"
		if query.SortOrder == "DESC" {
			order = "DESC"
		}
		sql += fmt.Sprintf(" ORDER BY r.%s %s", query.SortBy, order)
	} else {
		sql += " ORDER BY relevance DESC"
	}
	if query.Limit > 0 {
		sql += " LIMIT ?"
		args = append(args, query.Limit)
		if query.Offset > 0 {
			sql += " OFFSET ?"
			args = append(args, query.Offset)
		}
	}
	return idx.executeSearchQuery(ctx, sql, args...)
}
func (idx *SQLiteIndexer) searchStructured(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	sql := "SELECT cid, collection, rkey, record_type, data, created_at, updated_at FROM records WHERE 1=1"
	args := []interface{}{}
	if query.Collection != "" {
		sql += " AND collection = ?"
		args = append(args, query.Collection)
	}
	if query.RecordType != "" {
		sql += " AND record_type = ?"
		args = append(args, query.RecordType)
	}
	if len(query.Filters) > 0 {
		for attr, value := range query.Filters {
			sql += " AND cid IN (SELECT cid FROM record_attributes WHERE attribute_name = ? AND attribute_value = ?)"
			args = append(args, attr, fmt.Sprintf("%v", value))
		}
	}
	if query.SortBy != "" {
		order := "ASC"
		if query.SortOrder == "DESC" {
			order = "DESC"
		}
		sql += fmt.Sprintf(" ORDER BY %s %s", query.SortBy, order)
	} else {
		sql += " ORDER BY created_at DESC"
	}
	if query.Limit > 0 {
		sql += " LIMIT ?"
		args = append(args, query.Limit)
		if query.Offset > 0 {
			sql += " OFFSET ?"
			args = append(args, query.Offset)
		}
	}
	return idx.executeSearchQuery(ctx, sql, args...)
}
func (idx *SQLiteIndexer) executeSearchQuery(ctx context.Context, sql string, args ...interface{}) ([]SearchResult, error) {
	rows, err := idx.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var cidStr, dataJSON string
		var relevance *float64
		if strings.Contains(sql, "relevance") {
			err = rows.Scan(&cidStr, &result.Collection, &result.RKey, &result.RecordType,
				&dataJSON, &result.CreatedAt, &result.UpdatedAt, &relevance)
			if relevance != nil {
				result.Relevance = *relevance
			}
		} else {
			err = rows.Scan(&cidStr, &result.Collection, &result.RKey, &result.RecordType,
				&dataJSON, &result.CreatedAt, &result.UpdatedAt)
		}
		if err != nil {
			return nil, err
		}
		if result.CID, err = cid.Parse(cidStr); err != nil {
			return nil, fmt.Errorf("invalid CID in search results: %w", err)
		}
		if err = json.Unmarshal([]byte(dataJSON), &result.Data); err != nil {
			return nil, fmt.Errorf("invalid JSON data in search results: %w", err)
		}
		results = append(results, result)
	}
	return results, rows.Err()
}
func (idx *SQLiteIndexer) GetCollectionStats(ctx context.Context, collection string) (map[string]interface{}, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	row := idx.db.QueryRowContext(ctx, `
		SELECT record_count, type_count, first_record, last_updated
		FROM collection_stats 
		WHERE collection = ?
	`, collection)
	var recordCount, typeCount int
	var firstRecord, lastUpdated time.Time
	err := row.Scan(&recordCount, &typeCount, &firstRecord, &lastUpdated)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{
				"record_count": 0,
				"type_count":   0,
			}, nil
		}
		return nil, err
	}
	return map[string]interface{}{
		"record_count": recordCount,
		"type_count":   typeCount,
		"first_record": firstRecord,
		"last_updated": lastUpdated,
	}, nil
}
func (idx *SQLiteIndexer) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.db.Close()
}
