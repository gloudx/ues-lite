package repository

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"ues-lite/blockstore"
	"ues-lite/datastore"
	"ues-lite/headstorage"
	"ues-lite/indexer"
	"ues-lite/lexicon"
	"ues-lite/mst"
	"ues-lite/sqliteindexer"

	"github.com/ipfs/go-cid"
	badger4 "github.com/ipfs/go-ds-badger4"
	"github.com/ipld/go-ipld-prime/datamodel"
)

type Repository struct {
	bs          blockstore.Blockstore
	index       *indexer.Index
	sqliteIndex *sqliteindexer.SimpleSQLiteIndexer
	lexicon     *lexicon.Registry
	headStorage headstorage.HeadStorage
	headstorage.RepositoryState
	mu sync.RWMutex
}

func NewRepository(dataPath, sqliteDBPath, lexiconPath, repoID string) (*Repository, error) {
	ctx := context.Background()
	ds, err := datastore.NewDatastorage(dataPath, &badger4.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create datastore: %w", err)
	}
	bs := blockstore.NewBlockstore(ds)
	hStorage := headstorage.NewHeadStorage(ds)
	state, err := hStorage.LoadHead(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to load head state: %w", err)
	}
	index := indexer.NewIndex(bs, state.Head)
	sqliteIndex, err := sqliteindexer.NewSimpleSQLiteIndexer(sqliteDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite indexer: %w", err)
	}
	lex := lexicon.NewRegistry(lexiconPath)
	return &Repository{
		bs:              bs,
		index:           index,
		sqliteIndex:     sqliteIndex,
		lexicon:         lex,
		headStorage:     hStorage,
		RepositoryState: state,
	}, nil
}
func (r *Repository) Commit(ctx context.Context) error {
	if r.headStorage == nil {
		return nil
	}
	r.mu.RLock()
	state := headstorage.RepositoryState{
		Head:      r.Head,
		Prev:      r.Prev,
		RootIndex: r.index.Root(),
		Version:   1,
		RepoID:    r.RepoID,
	}
	r.mu.RUnlock()
	return r.headStorage.SaveHead(ctx, r.RepoID, state)
}
func (r *Repository) PutRecord(ctx context.Context, collection, rkey string, node datamodel.Node) (cid.Cid, error) {
	if r.lexicon != nil {
		if err := r.validateRecordWithLexicon(ctx, collection, node); err != nil {
			return cid.Undef, fmt.Errorf("lexicon validation failed for %s/%s: %w", collection, rkey, err)
		}
	}
	valueCID, err := r.bs.PutNode(ctx, node)
	if err != nil {
		return cid.Undef, fmt.Errorf("store record node: %w", err)
	}
	headCid, err := r.index.Put(ctx, collection, rkey, valueCID)
	if err != nil {
		return cid.Undef, err
	}
	if r.sqliteIndex != nil {
		if err := r.indexRecordInSQLite(ctx, valueCID, collection, rkey, node); err != nil {
			fmt.Printf("Warning: SQLite indexing failed for %s/%s: %v\n", collection, rkey, err)
		}
	}
	r.Head = headCid
	if err := r.Commit(ctx); err != nil {
		return cid.Undef, fmt.Errorf("commit after put record: %w", err)
	}
	return valueCID, nil
}
func (r *Repository) indexRecordInSQLite(ctx context.Context, recordCID cid.Cid, collection, rkey string, node datamodel.Node) error {
	data, err := extractDataFromNode(node)
	if err != nil {
		return fmt.Errorf("failed to extract data from node: %w", err)
	}
	searchText := generateSearchText(data)
	metadata := sqliteindexer.IndexMetadata{
		Collection: collection,
		RKey:       rkey,
		RecordType: inferRecordType(collection, data),
		Data:       data,
		SearchText: searchText,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	return r.sqliteIndex.IndexRecord(ctx, recordCID, metadata)
}
func extractDataFromNode(node datamodel.Node) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	iterator := node.MapIterator()
	for !iterator.Done() {
		key, value, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		keyStr, err := key.AsString()
		if err != nil {
			continue
		}
		goValue, err := nodeToGoValue(value)
		if err != nil {
			continue
		}
		result[keyStr] = goValue
	}
	return result, nil
}
func nodeToGoValue(node datamodel.Node) (interface{}, error) {
	switch node.Kind() {
	case datamodel.Kind_String:
		return node.AsString()
	case datamodel.Kind_Bool:
		return node.AsBool()
	case datamodel.Kind_Int:
		return node.AsInt()
	case datamodel.Kind_Float:
		return node.AsFloat()
	case datamodel.Kind_List:
		var result []interface{}
		iterator := node.ListIterator()
		for !iterator.Done() {
			_, value, err := iterator.Next()
			if err != nil {
				return nil, err
			}
			goValue, err := nodeToGoValue(value)
			if err != nil {
				continue
			}
			result = append(result, goValue)
		}
		return result, nil
	case datamodel.Kind_Map:
		result := make(map[string]interface{})
		iterator := node.MapIterator()
		for !iterator.Done() {
			key, value, err := iterator.Next()
			if err != nil {
				return nil, err
			}
			keyStr, err := key.AsString()
			if err != nil {
				continue
			}
			goValue, err := nodeToGoValue(value)
			if err != nil {
				continue
			}
			result[keyStr] = goValue
		}
		return result, nil
	default:
		return fmt.Sprintf("%v", node), nil
	}
}
func inferRecordType(collection string, data map[string]interface{}) string {
	if recordType, exists := data["$type"]; exists {
		if typeStr, ok := recordType.(string); ok {
			return typeStr
		}
	}
	switch collection {
	case "posts", "app.bsky.feed.post":
		return "post"
	case "follows", "app.bsky.graph.follow":
		return "follow"
	case "likes", "app.bsky.feed.like":
		return "like"
	case "profiles", "app.bsky.actor.profile":
		return "profile"
	default:
		return "record"
	}
}
func generateSearchText(data map[string]interface{}) string {
	var parts []string
	for key, value := range data {
		parts = append(parts, key)
		switch v := value.(type) {
		case string:
			parts = append(parts, v)
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					parts = append(parts, str)
				}
			}
		case map[string]interface{}:
			for _, nested := range v {
				if str, ok := nested.(string); ok {
					parts = append(parts, str)
				}
			}
		}
	}
	return strings.Join(parts, " ")
}
func (r *Repository) validateRecordWithLexicon(ctx context.Context, collection string, node datamodel.Node) error {
	lexiconID := inferLexiconID(collection)
	definition, err := r.lexicon.GetSchema(lexiconID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("failed to get lexicon %s: %w", lexiconID, err)
	}
	if definition.Status == lexicon.SchemaStatusArchived {
		return fmt.Errorf("lexicon %s is archived and cannot be used", lexiconID)
	}
	if definition.Status == lexicon.SchemaStatusDeprecated {
		return fmt.Errorf("lexicon %s is deprecated", lexiconID)
	}
	if err := r.lexicon.ValidateData(lexiconID, node); err != nil {
		return fmt.Errorf("data validation failed: %w", err)
	}
	return nil
}
func inferLexiconID(collection string) string {
	return collection
}
func (r *Repository) DeleteRecord(ctx context.Context, collection, rkey string) (bool, error) {
	var recordCID cid.Cid
	if r.sqliteIndex != nil {
		if cid, found, err := r.index.Get(ctx, collection, rkey); err == nil && found {
			recordCID = cid
		}
	}
	_, removed, err := r.index.Delete(ctx, collection, rkey)
	if err != nil {
		return false, err
	}
	if r.sqliteIndex != nil && removed && recordCID != cid.Undef {
		if err := r.sqliteIndex.DeleteRecord(ctx, recordCID); err != nil {
			fmt.Printf("Warning: SQLite deletion failed for %s/%s: %v\n", collection, rkey, err)
		}
	}
	return removed, nil
}
func (r *Repository) GetRecordCID(ctx context.Context, collection, rkey string) (cid.Cid, bool, error) {
	return r.index.Get(ctx, collection, rkey)
}
func (r *Repository) ListCollection(ctx context.Context, collection string) ([]cid.Cid, error) {
	entries, err := r.index.ListCollection(ctx, collection)
	if err != nil {
		return nil, err
	}
	out := make([]cid.Cid, len(entries))
	for i, entry := range entries {
		out[i] = entry.Value
	}
	return out, nil
}
func (r *Repository) SearchRecords(ctx context.Context, query sqliteindexer.SearchQuery) ([]sqliteindexer.SearchResult, error) {
	if r.sqliteIndex == nil {
		return nil, fmt.Errorf("SQLite indexer is not enabled for this repository")
	}
	return r.sqliteIndex.SearchRecords(ctx, query)
}
func (r *Repository) GetCollectionStats(ctx context.Context, collection string) (map[string]interface{}, error) {
	if r.sqliteIndex == nil {
		return nil, fmt.Errorf("SQLite indexer is not enabled for this repository")
	}
	return r.sqliteIndex.GetCollectionStats(ctx, collection)
}
func (r *Repository) HasSQLiteIndex() bool {
	return r.sqliteIndex != nil
}
func (r *Repository) CloseSQLiteIndex() error {
	if r.sqliteIndex == nil {
		return nil
	}
	err := r.sqliteIndex.Close()
	r.sqliteIndex = nil
	return err
}
func (r *Repository) CreateCollection(ctx context.Context, name string) (cid.Cid, error) {
	return r.index.CreateCollection(ctx, name)
}
func (r *Repository) DeleteCollection(ctx context.Context, name string) (cid.Cid, error) {
	return r.index.DeleteCollection(ctx, name)
}
func (r *Repository) HasCollection(name string) bool {
	return r.index.HasCollection(name)
}
func (r *Repository) ListCollections() []string {
	return r.index.Collections()
}
func (r *Repository) CollectionRoot(name string) (cid.Cid, bool) {
	return r.index.CollectionRoot(name)
}
func (r *Repository) CollectionRootHash(ctx context.Context, name string) ([]byte, bool, error) {
	return r.index.CollectionRootHash(ctx, name)
}
func (r *Repository) GetRecord(ctx context.Context, collection, rkey string) (datamodel.Node, bool, error) {
	c, ok, err := r.index.Get(ctx, collection, rkey)
	if err != nil || !ok {
		return nil, ok, err
	}
	n, err := r.bs.GetNode(ctx, c)
	if err != nil {
		return nil, false, err
	}
	return n, true, nil
}
func (r *Repository) ListRecords(ctx context.Context, collection string) ([]mst.Entry, error) {
	return r.index.ListCollection(ctx, collection)
}
func (r *Repository) InclusionPath(ctx context.Context, collection, rkey string) ([]cid.Cid, bool, error) {
	return r.index.InclusionPath(ctx, collection, rkey)
}
func (r *Repository) ExportCollectionCAR(ctx context.Context, collection string, w io.Writer) error {
	root, ok := r.index.CollectionRoot(collection)
	if !ok {
		return fmt.Errorf("collection not found: %s", collection)
	}
	if !root.Defined() {
		return fmt.Errorf("collection is empty: %s", collection)
	}
	selectorNode := blockstore.BuildSelectorNodeExploreAll()
	return r.bs.ExportCARV2(ctx, root, selectorNode, w)
}
func (r *Repository) Close() error {
	var firstErr error
	if r.sqliteIndex != nil {
		if err := r.sqliteIndex.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close SQLite indexer: %w", err)
		}
		r.sqliteIndex = nil
	}
	if r.index != nil {
		if err := r.index.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close index: %w", err)
		}
		r.index = nil
	}
	if r.bs != nil {
		if err := r.bs.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close blockstore: %w", err)
		}
		r.bs = nil
	}
	return firstErr
}
func (r *Repository) Datastore() datastore.Datastore {
	return r.bs.Datastore()
}
