package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

// StreamFormat определяет формат стрима
type StreamFormat string

const (
	StreamFormatJSON     StreamFormat = "json"
	StreamFormatJSONL    StreamFormat = "jsonl"     // JSON Lines
	StreamFormatCSV      StreamFormat = "csv"
	StreamFormatSSE      StreamFormat = "sse"       // Server-Sent Events
	StreamFormatBinary   StreamFormat = "binary"
	StreamFormatXML      StreamFormat = "xml"
	StreamFormatYAML     StreamFormat = "yaml"
)

// StreamOptions конфигурирует параметры стрима
type StreamOptions struct {
	Format           StreamFormat      `json:"format"`
	Prefix           ds.Key            `json:"prefix"`
	JQFilter         string            `json:"jq_filter"`
	Limit            int               `json:"limit"`
	BufferSize       int               `json:"buffer_size"`
	FlushInterval    time.Duration     `json:"flush_interval"`
	IncludeKeys      bool              `json:"include_keys"`
	TreatAsString    bool              `json:"treat_as_string"`
	IgnoreErrors     bool              `json:"ignore_errors"`
	Headers          map[string]string `json:"headers"`
	Compression      string            `json:"compression"` // gzip, deflate, none
	ChunkSize        int               `json:"chunk_size"`
	Timeout          time.Duration     `json:"timeout"`
}

// StreamWriter интерфейс для записи стрима
type StreamWriter interface {
	io.Writer
	WriteHeader(headers map[string]string) error
	WriteRecord(key ds.Key, value interface{}) error
	WriteEvent(eventType string, data interface{}) error
	Flush() error
	Close() error
	SetMetadata(metadata map[string]interface{}) error
}

// StreamReader интерфейс для чтения стрима
type StreamReader interface {
	io.Reader
	ReadRecord() (key ds.Key, value interface{}, err error)
	ReadEvent() (eventType string, data interface{}, err error)
	Close() error
	GetMetadata() map[string]interface{}
}

// JSONStreamWriter пишет данные в JSON формате
type JSONStreamWriter struct {
	writer      io.Writer
	isFirst     bool
	mu          sync.Mutex
	metadata    map[string]interface{}
	includeKeys bool
}

func NewJSONStreamWriter(writer io.Writer, includeKeys bool) *JSONStreamWriter {
	w := &JSONStreamWriter{
		writer:      writer,
		isFirst:     true,
		metadata:    make(map[string]interface{}),
		includeKeys: includeKeys,
	}
	w.writer.Write([]byte("["))
	return w
}

func (w *JSONStreamWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w *JSONStreamWriter) WriteHeader(headers map[string]string) error {
	return nil // JSON не поддерживает заголовки напрямую
}

func (w *JSONStreamWriter) WriteRecord(key ds.Key, value interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isFirst {
		w.writer.Write([]byte(","))
	}
	w.isFirst = false

	var record interface{}
	if w.includeKeys {
		record = map[string]interface{}{
			"key":   key.String(),
			"value": value,
		}
	} else {
		record = value
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	_, err = w.writer.Write(data)
	return err
}

func (w *JSONStreamWriter) WriteEvent(eventType string, data interface{}) error {
	return w.WriteRecord(ds.NewKey("/event/"+eventType), data)
}

func (w *JSONStreamWriter) Flush() error {
	return nil
}

func (w *JSONStreamWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.writer.Write([]byte("]"))
	return err
}

func (w *JSONStreamWriter) SetMetadata(metadata map[string]interface{}) error {
	w.metadata = metadata
	return nil
}

// JSONLStreamWriter пишет данные в JSON Lines формате
type JSONLStreamWriter struct {
	writer      io.Writer
	mu          sync.Mutex
	metadata    map[string]interface{}
	includeKeys bool
}

func NewJSONLStreamWriter(writer io.Writer, includeKeys bool) *JSONLStreamWriter {
	return &JSONLStreamWriter{
		writer:      writer,
		metadata:    make(map[string]interface{}),
		includeKeys: includeKeys,
	}
}

func (w *JSONLStreamWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w *JSONLStreamWriter) WriteHeader(headers map[string]string) error {
	return nil
}

func (w *JSONLStreamWriter) WriteRecord(key ds.Key, value interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var record interface{}
	if w.includeKeys {
		record = map[string]interface{}{
			"key":   key.String(),
			"value": value,
		}
	} else {
		record = value
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	_, err = w.writer.Write(append(data, '\n'))
	return err
}

func (w *JSONLStreamWriter) WriteEvent(eventType string, data interface{}) error {
	return w.WriteRecord(ds.NewKey("/event/"+eventType), data)
}

func (w *JSONLStreamWriter) Flush() error {
	return nil
}

func (w *JSONLStreamWriter) Close() error {
	return nil
}

func (w *JSONLStreamWriter) SetMetadata(metadata map[string]interface{}) error {
	w.metadata = metadata
	return nil
}

// SSEStreamWriter пишет данные в Server-Sent Events формате
type SSEStreamWriter struct {
	writer   io.Writer
	mu       sync.Mutex
	metadata map[string]interface{}
}

func NewSSEStreamWriter(writer io.Writer) *SSEStreamWriter {
	return &SSEStreamWriter{
		writer:   writer,
		metadata: make(map[string]interface{}),
	}
}

func (w *SSEStreamWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w *SSEStreamWriter) WriteHeader(headers map[string]string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for key, value := range headers {
		if strings.ToLower(key) == "content-type" {
			continue // SSE устанавливает свой content-type
		}
		w.writer.Write([]byte(fmt.Sprintf(": %s: %s\n", key, value)))
	}
	return nil
}

func (w *SSEStreamWriter) WriteRecord(key ds.Key, value interface{}) error {
	data := map[string]interface{}{
		"key":   key.String(),
		"value": value,
	}
	return w.WriteEvent("data", data)
}

func (w *SSEStreamWriter) WriteEvent(eventType string, data interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Записываем SSE формат
	if eventType != "" {
		w.writer.Write([]byte(fmt.Sprintf("event: %s\n", eventType)))
	}
	
	// Разбиваем данные на строки для SSE
	dataStr := string(jsonData)
	for _, line := range strings.Split(dataStr, "\n") {
		w.writer.Write([]byte(fmt.Sprintf("data: %s\n", line)))
	}
	
	w.writer.Write([]byte("\n"))
	return nil
}

func (w *SSEStreamWriter) Flush() error {
	return nil
}

func (w *SSEStreamWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.writer.Write([]byte("event: close\ndata: \n\n"))
	return err
}

func (w *SSEStreamWriter) SetMetadata(metadata map[string]interface{}) error {
	w.metadata = metadata
	return nil
}

// CSVStreamWriter пишет данные в CSV формате
type CSVStreamWriter struct {
	writer       io.Writer
	mu           sync.Mutex
	metadata     map[string]interface{}
	headerWritten bool
	includeKeys  bool
}

func NewCSVStreamWriter(writer io.Writer, includeKeys bool) *CSVStreamWriter {
	return &CSVStreamWriter{
		writer:      writer,
		metadata:    make(map[string]interface{}),
		includeKeys: includeKeys,
	}
}

func (w *CSVStreamWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w *CSVStreamWriter) WriteHeader(headers map[string]string) error {
	return nil
}

func (w *CSVStreamWriter) WriteRecord(key ds.Key, value interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.headerWritten {
		if w.includeKeys {
			w.writer.Write([]byte("key,value\n"))
		} else {
			w.writer.Write([]byte("value\n"))
		}
		w.headerWritten = true
	}

	if w.includeKeys {
		valueStr := w.valueToCSVString(value)
		_, err := w.writer.Write([]byte(fmt.Sprintf("%s,%s\n", w.escapeCSV(key.String()), valueStr)))
		return err
	} else {
		valueStr := w.valueToCSVString(value)
		_, err := w.writer.Write([]byte(fmt.Sprintf("%s\n", valueStr)))
		return err
	}
}

func (w *CSVStreamWriter) valueToCSVString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return w.escapeCSV(v)
	case []byte:
		return w.escapeCSV(string(v))
	default:
		jsonData, _ := json.Marshal(v)
		return w.escapeCSV(string(jsonData))
	}
}

func (w *CSVStreamWriter) escapeCSV(s string) string {
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

func (w *CSVStreamWriter) WriteEvent(eventType string, data interface{}) error {
	return w.WriteRecord(ds.NewKey("/event/"+eventType), data)
}

func (w *CSVStreamWriter) Flush() error {
	return nil
}

func (w *CSVStreamWriter) Close() error {
	return nil
}

func (w *CSVStreamWriter) SetMetadata(metadata map[string]interface{}) error {
	w.metadata = metadata
	return nil
}

// BinaryStreamWriter пишет данные в бинарном формате
type BinaryStreamWriter struct {
	writer   io.Writer
	mu       sync.Mutex
	metadata map[string]interface{}
}

func NewBinaryStreamWriter(writer io.Writer) *BinaryStreamWriter {
	return &BinaryStreamWriter{
		writer:   writer,
		metadata: make(map[string]interface{}),
	}
}

func (w *BinaryStreamWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w *BinaryStreamWriter) WriteHeader(headers map[string]string) error {
	return nil
}

func (w *BinaryStreamWriter) WriteRecord(key ds.Key, value interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Записываем длину ключа, ключ, длину значения, значение
	keyBytes := []byte(key.String())
	
	var valueBytes []byte
	switch v := value.(type) {
	case []byte:
		valueBytes = v
	case string:
		valueBytes = []byte(v)
	default:
		var err error
		valueBytes, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}

	// Записываем в формате: [keyLen:4][key][valueLen:4][value]
	if err := w.writeUint32(uint32(len(keyBytes))); err != nil {
		return err
	}
	if _, err := w.writer.Write(keyBytes); err != nil {
		return err
	}
	if err := w.writeUint32(uint32(len(valueBytes))); err != nil {
		return err
	}
	_, err := w.writer.Write(valueBytes)
	return err
}

func (w *BinaryStreamWriter) writeUint32(value uint32) error {
	bytes := []byte{
		byte(value >> 24),
		byte(value >> 16),
		byte(value >> 8),
		byte(value),
	}
	_, err := w.writer.Write(bytes)
	return err
}

func (w *BinaryStreamWriter) WriteEvent(eventType string, data interface{}) error {
	return w.WriteRecord(ds.NewKey("/event/"+eventType), data)
}

func (w *BinaryStreamWriter) Flush() error {
	return nil
}

func (w *BinaryStreamWriter) Close() error {
	return nil
}

func (w *BinaryStreamWriter) SetMetadata(metadata map[string]interface{}) error {
	w.metadata = metadata
	return nil
}

// Методы для datastore
func (s *datastorage) StreamTo(ctx context.Context, writer io.Writer, opts *StreamOptions) error {
	if opts == nil {
		opts = &StreamOptions{
			Format:     StreamFormatJSON,
			Prefix:     ds.NewKey("/"),
			BufferSize: 1000,
			Timeout:    30 * time.Second,
		}
	}

	streamCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	streamWriter := s.createStreamWriter(writer, opts)
	defer streamWriter.Close()

	if opts.Headers != nil {
		if err := streamWriter.WriteHeader(opts.Headers); err != nil {
			return fmt.Errorf("ошибка записи заголовков: %w", err)
		}
	}

	if opts.JQFilter != "" {
		return s.streamWithJQFilter(streamCtx, streamWriter, opts)
	}

	return s.streamDirect(streamCtx, streamWriter, opts)
}

func (s *datastorage) createStreamWriter(writer io.Writer, opts *StreamOptions) StreamWriter {
	switch opts.Format {
	case StreamFormatJSON:
		return NewJSONStreamWriter(writer, opts.IncludeKeys)
	case StreamFormatJSONL:
		return NewJSONLStreamWriter(writer, opts.IncludeKeys)
	case StreamFormatSSE:
		return NewSSEStreamWriter(writer)
	case StreamFormatCSV:
		return NewCSVStreamWriter(writer, opts.IncludeKeys)
	case StreamFormatBinary:
		return NewBinaryStreamWriter(writer)
	default:
		return NewJSONStreamWriter(writer, opts.IncludeKeys)
	}
}

func (s *datastorage) streamDirect(ctx context.Context, writer StreamWriter, opts *StreamOptions) error {
	q := query.Query{
		Prefix: opts.Prefix.String(),
	}

	results, err := s.Datastore.Query(ctx, q)
	if err != nil {
		return fmt.Errorf("ошибка запроса: %w", err)
	}
	defer results.Close()

	count := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res, ok := <-results.Next():
			if !ok {
				return nil
			}
			if res.Error != nil {
				if opts.IgnoreErrors {
					continue
				}
				return res.Error
			}

			if opts.Limit > 0 && count >= opts.Limit {
				return nil
			}

			key := ds.NewKey(res.Key)
			var value interface{}

			if opts.TreatAsString {
				value = string(res.Value)
			} else {
				if err := json.Unmarshal(res.Value, &value); err != nil {
					if opts.IgnoreErrors {
						value = string(res.Value)
					} else {
						return fmt.Errorf("ошибка парсинга JSON для ключа %s: %w", key.String(), err)
					}
				}
			}

			if err := writer.WriteRecord(key, value); err != nil {
				return fmt.Errorf("ошибка записи записи: %w", err)
			}

			count++
		}
	}
}

func (s *datastorage) streamWithJQFilter(ctx context.Context, writer StreamWriter, opts *StreamOptions) error {
	jqOpts := &JQQueryOptions{
		Prefix:           opts.Prefix,
		Limit:            opts.Limit,
		TreatAsString:    opts.TreatAsString,
		IgnoreParseError: opts.IgnoreErrors,
		Timeout:          opts.Timeout,
	}

	resultChan, errorChan, err := s.QueryJQ(ctx, opts.JQFilter, jqOpts)
	if err != nil {
		return fmt.Errorf("ошибка JQ запроса: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-errorChan:
			if ok && err != nil {
				if opts.IgnoreErrors {
					continue
				}
				return err
			}
		case result, ok := <-resultChan:
			if !ok {
				return nil
			}

			if err := writer.WriteRecord(result.Key, result.Value); err != nil {
				return fmt.Errorf("ошибка записи записи: %w", err)
			}
		}
	}
}

// StreamEvents стримит события в реальном времени
func (s *datastorage) StreamEvents(ctx context.Context, writer io.Writer, opts *StreamOptions) error {
	if opts == nil {
		opts = &StreamOptions{
			Format:     StreamFormatSSE,
			BufferSize: 100,
		}
	}

	streamWriter := s.createStreamWriter(writer, opts)
	defer streamWriter.Close()

	if opts.Headers != nil {
		if err := streamWriter.WriteHeader(opts.Headers); err != nil {
			return fmt.Errorf("ошибка записи заголовков: %w", err)
		}
	}

	// Создаем подписчика для событий
	subscriber := s.SubscribeChannel("stream-events", opts.BufferSize)
	defer s.Unsubscribe(subscriber.ID())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-subscriber.Events():
			if !ok {
				return nil
			}

			eventType := "unknown"
			switch event.Type {
			case EventPut:
				eventType = "put"
			case EventDelete:
				eventType = "delete"
			case EventBatch:
				eventType = "batch"
			}

			eventData := map[string]interface{}{
				"key":       event.Key.String(),
				"value":     string(event.Value),
				"timestamp": event.Timestamp,
			}

			if err := streamWriter.WriteEvent(eventType, eventData); err != nil {
				return fmt.Errorf("ошибка записи события: %w", err)
			}

			if err := streamWriter.Flush(); err != nil {
				return fmt.Errorf("ошибка flush: %w", err)
			}
		}
	}
}

// Удобные методы для разных типов стримминга
func (s *datastorage) StreamJSON(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error {
	opts := &StreamOptions{
		Format:      StreamFormatJSON,
		Prefix:      prefix,
		IncludeKeys: includeKeys,
	}
	return s.StreamTo(ctx, writer, opts)
}

func (s *datastorage) StreamJSONL(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error {
	opts := &StreamOptions{
		Format:      StreamFormatJSONL,
		Prefix:      prefix,
		IncludeKeys: includeKeys,
	}
	return s.StreamTo(ctx, writer, opts)
}

func (s *datastorage) StreamCSV(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error {
	opts := &StreamOptions{
		Format:      StreamFormatCSV,
		Prefix:      prefix,
		IncludeKeys: includeKeys,
	}
	return s.StreamTo(ctx, writer, opts)
}

func (s *datastorage) StreamSSE(ctx context.Context, writer io.Writer, headers map[string]string) error {
	opts := &StreamOptions{
		Format:  StreamFormatSSE,
		Headers: headers,
	}
	return s.StreamEvents(ctx, writer, opts)
}

func (s *datastorage) StreamBinary(ctx context.Context, writer io.Writer, prefix ds.Key) error {
	opts := &StreamOptions{
		Format: StreamFormatBinary,
		Prefix: prefix,
	}
	return s.StreamTo(ctx, writer, opts)
}

func (s *datastorage) StreamWithJQ(ctx context.Context, writer io.Writer, jqQuery string, format StreamFormat, prefix ds.Key) error {
	opts := &StreamOptions{
		Format:   format,
		Prefix:   prefix,
		JQFilter: jqQuery,
	}
	return s.StreamTo(ctx, writer, opts)
}

// StreamPipeline создает пайплайн для комплексной обработки и стримминга
type StreamPipeline struct {
	ds       *datastorage
	stages   []PipelineStage
	opts     *StreamOptions
}

type PipelineStage interface {
	Process(ctx context.Context, key ds.Key, value interface{}) (interface{}, error)
}

type JQPipelineStage struct {
	query string
	ds    *datastorage
}

func (s *JQPipelineStage) Process(ctx context.Context, key ds.Key, value interface{}) (interface{}, error) {
	s.ds.initJQCache()
	
	compiled, exists := s.ds.jqCache.get(s.query)
	if !exists {
		var err error
		compiled, err = s.ds.compileJQ(s.query)
		if err != nil {
			return nil, err
		}
		s.ds.jqCache.set(s.query, compiled)
	}
	
	return compiled.Execute(value)
}

func (s *datastorage) NewStreamPipeline(opts *StreamOptions) *StreamPipeline {
	return &StreamPipeline{
		ds:     s,
		stages: make([]PipelineStage, 0),
		opts:   opts,
	}
}

func (p *StreamPipeline) AddJQStage(jqQuery string) *StreamPipeline {
	stage := &JQPipelineStage{
		query: jqQuery,
		ds:    p.ds,
	}
	p.stages = append(p.stages, stage)
	return p
}

func (p *StreamPipeline) Stream(ctx context.Context, writer io.Writer) error {
	if len(p.stages) == 0 {
		return p.ds.StreamTo(ctx, writer, p.opts)
	}
	
	streamWriter := p.ds.createStreamWriter(writer, p.opts)
	defer streamWriter.Close()
	
	if p.opts.Headers != nil {
		if err := streamWriter.WriteHeader(p.opts.Headers); err != nil {
			return fmt.Errorf("ошибка записи заголовков: %w", err)
		}
	}
	
	q := query.Query{
		Prefix: p.opts.Prefix.String(),
	}
	
	results, err := p.ds.Datastore.Query(ctx, q)
	if err != nil {
		return fmt.Errorf("ошибка запроса: %w", err)
	}
	defer results.Close()
	
	count := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res, ok := <-results.Next():
			if !ok {
				return nil
			}
			if res.Error != nil {
				if p.opts.IgnoreErrors {
					continue
				}
				return res.Error
			}
			
			if p.opts.Limit > 0 && count >= p.opts.Limit {
				return nil
			}
			
			key := ds.NewKey(res.Key)
			var value interface{}
			
			if p.opts.TreatAsString {
				value = string(res.Value)
			} else {
				if err := json.Unmarshal(res.Value, &value); err != nil {
					if p.opts.IgnoreErrors {
						value = string(res.Value)
					} else {
						return fmt.Errorf("ошибка парсинга JSON для ключа %s: %w", key.String(), err)
					}
				}
			}
			
			// Применяем все стадии пайплайна
			processedValue := value
			for _, stage := range p.stages {
				processedValue, err = stage.Process(ctx, key, processedValue)
				if err != nil {
					if p.opts.IgnoreErrors {
						continue
					}
					return fmt.Errorf("ошибка обработки стадии пайплайна: %w", err)
				}
			}
			
			if err := streamWriter.WriteRecord(key, processedValue); err != nil {
				return fmt.Errorf("ошибка записи записи: %w", err)
			}
			
			count++
		}
	}
}