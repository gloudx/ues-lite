package indexer
import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"ues-lite/blockstore"
	"ues-lite/mst"
)
type Index struct {
	bs	blockstore.Blockstore
	mu	sync.RWMutex
	root	cid.Cid			
	roots	map[string]cid.Cid	
}
func NewIndex(bs blockstore.Blockstore, root cid.Cid) *Index {
	return &Index{
		bs:	bs,
		root:	root,
		roots:	make(map[string]cid.Cid),
	}
}
func (i *Index) Load(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.root.Defined() {
		return nil
	}
	i.roots = make(map[string]cid.Cid)
	dm, err := i.bs.GetNode(ctx, i.root)
	if err != nil {
		return fmt.Errorf("index: load root node: %w", err)
	}
	it := dm.MapIterator()
	for !it.Done() {
		k, v, err := it.Next()
		if err != nil {
			return fmt.Errorf("index: iterate map: %w", err)
		}
		name, err := k.AsString()
		if err != nil {
			return fmt.Errorf("index: invalid key type: %w", err)
		}
		if v.IsNull() {
			i.roots[name] = cid.Undef
			continue
		}
		lnk, err := v.AsLink()
		if err != nil {
			return fmt.Errorf("index: value is not link: %w", err)
		}
		cl, ok := lnk.(cidlink.Link)
		if !ok {
			return errors.New("index: unexpected link type")
		}
		i.roots[name] = cl.Cid
	}
	return nil
}
func (i *Index) Root() cid.Cid {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.root
}
func (i *Index) materialize(ctx context.Context) (cid.Cid, error) {
	i.mu.RLock()
	keys := make([]string, 0, len(i.roots))
	for k := range i.roots {
		keys = append(keys, k)
	}
	i.mu.RUnlock()
	sort.Strings(keys)
	b := basicnode.Prototype.Map.NewBuilder()
	ma, err := b.BeginMap(int64(len(keys)))
	if err != nil {
		return cid.Undef, err
	}
	for _, name := range keys {
		entry, err := ma.AssembleEntry(name)
		if err != nil {
			return cid.Undef, err
		}
		i.mu.RLock()
		root := i.roots[name]
		i.mu.RUnlock()
		if root.Defined() {
			if err := entry.AssignLink(cidlink.Link{Cid: root}); err != nil {
				return cid.Undef, err
			}
		} else {
			if err := entry.AssignNull(); err != nil {
				return cid.Undef, err
			}
		}
	}
	if err := ma.Finish(); err != nil {
		return cid.Undef, err
	}
	n := b.Build()
	c, err := i.bs.PutNode(ctx, n)
	if err != nil {
		return cid.Undef, err
	}
	i.mu.Lock()
	i.root = c
	i.mu.Unlock()
	return c, nil
}
func (i *Index) CreateCollection(ctx context.Context, name string) (cid.Cid, error) {
	i.mu.Lock()
	if _, exists := i.roots[name]; exists {
		i.mu.Unlock()
		return i.root, fmt.Errorf("collection already exists: %s", name)
	}
	i.roots[name] = cid.Undef
	i.mu.Unlock()
	return i.materialize(ctx)
}
func (i *Index) DeleteCollection(ctx context.Context, name string) (cid.Cid, error) {
	i.mu.Lock()
	if _, exists := i.roots[name]; !exists {
		i.mu.Unlock()
		return i.root, fmt.Errorf("collection not found: %s", name)
	}
	delete(i.roots, name)
	i.mu.Unlock()
	return i.materialize(ctx)
}
func (i *Index) HasCollection(name string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	_, ok := i.roots[name]
	return ok
}
func (i *Index) Collections() []string {
	i.mu.RLock()
	keys := make([]string, 0, len(i.roots))
	for k := range i.roots {
		keys = append(keys, k)
	}
	i.mu.RUnlock()
	sort.Strings(keys)
	return keys
}
func (i *Index) collectionRoot(name string) (cid.Cid, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	c, ok := i.roots[name]
	return c, ok
}
func (i *Index) Put(ctx context.Context, collection, rkey string, value cid.Cid) (cid.Cid, error) {
	i.mu.RLock()
	root, ok := i.roots[collection]
	i.mu.RUnlock()
	if !ok {
		return i.root, fmt.Errorf("collection not found: %s", collection)
	}
	tree := mst.NewTree(i.bs)
	if err := tree.Load(ctx, root); err != nil {
		return i.root, err
	}
	newRoot, err := tree.Put(ctx, rkey, value)
	if err != nil {
		return i.root, err
	}
	i.mu.Lock()
	i.roots[collection] = newRoot
	i.mu.Unlock()
	return i.materialize(ctx)
}
func (i *Index) Delete(ctx context.Context, collection, rkey string) (cid.Cid, bool, error) {
	i.mu.RLock()
	root, ok := i.roots[collection]
	i.mu.RUnlock()
	if !ok {
		return i.root, false, fmt.Errorf("collection not found: %s", collection)
	}
	tree := mst.NewTree(i.bs)
	if err := tree.Load(ctx, root); err != nil {
		return i.root, false, err
	}
	newRoot, removed, err := tree.Delete(ctx, rkey)
	if err != nil {
		return i.root, false, err
	}
	if !removed {
		return i.root, false, nil
	}
	i.mu.Lock()
	i.roots[collection] = newRoot
	i.mu.Unlock()
	c, err := i.materialize(ctx)
	return c, true, err
}
func (i *Index) Get(ctx context.Context, collection, rkey string) (cid.Cid, bool, error) {
	i.mu.RLock()
	root, ok := i.roots[collection]
	i.mu.RUnlock()
	if !ok {
		return cid.Undef, false, fmt.Errorf("collection not found: %s", collection)
	}
	tree := mst.NewTree(i.bs)
	if err := tree.Load(ctx, root); err != nil {
		return cid.Undef, false, err
	}
	return tree.Get(ctx, rkey)
}
func (i *Index) ListCollection(ctx context.Context, collection string) ([]mst.Entry, error) {
	i.mu.RLock()
	root, ok := i.roots[collection]
	i.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("collection not found: %s", collection)
	}
	if !root.Defined() {
		return []mst.Entry{}, nil
	}
	tree := mst.NewTree(i.bs)
	if err := tree.Load(ctx, root); err != nil {
		return nil, err
	}
	return tree.Range(ctx, "", "")
}
func (i *Index) CollectionRoot(name string) (cid.Cid, bool) {
	return i.collectionRoot(name)
}
func (i *Index) CollectionRootHash(ctx context.Context, name string) ([]byte, bool, error) {
	root, ok := i.collectionRoot(name)
	if !ok {
		return nil, false, fmt.Errorf("collection not found: %s", name)
	}
	if !root.Defined() {
		return nil, true, nil
	}
	n, err := i.bs.GetNode(ctx, root)
	if err != nil {
		return nil, false, err
	}
	hashNode, err := n.LookupByString("hash")
	if err != nil {
		return nil, false, fmt.Errorf("mst root missing hash: %w", err)
	}
	b, err := hashNode.AsBytes()
	if err != nil {
		return nil, false, fmt.Errorf("mst root invalid hash: %w", err)
	}
	return append([]byte(nil), b...), true, nil
}
func (i *Index) InclusionPath(ctx context.Context, name, rkey string) ([]cid.Cid, bool, error) {
	root, ok := i.collectionRoot(name)
	if !ok {
		return nil, false, fmt.Errorf("collection not found: %s", name)
	}
	if !root.Defined() {
		return []cid.Cid{}, false, nil
	}
	var path []cid.Cid
	cur := root
	for cur.Defined() {
		path = append(path, cur)
		dm, err := i.bs.GetNode(ctx, cur)
		if err != nil {
			return nil, false, err
		}
		kNode, err := dm.LookupByString("key")
		if err != nil {
			return nil, false, fmt.Errorf("mst node missing key: %w", err)
		}
		key, err := kNode.AsString()
		if err != nil {
			return nil, false, fmt.Errorf("mst node key type: %w", err)
		}
		switch cmp := compareStrings(rkey, key); {
		case cmp == 0:
			return path, true, nil
		case cmp < 0:
			left, _ := maybeLink(dm, "left")
			cur = left
		default:
			right, _ := maybeLink(dm, "right")
			cur = right
		}
	}
	return path, false, nil
}
func maybeLink(n datamodel.Node, field string) (cid.Cid, error) {
	child, err := n.LookupByString(field)
	if err != nil {
		return cid.Undef, nil
	}
	if child.IsNull() {
		return cid.Undef, nil
	}
	l, err := child.AsLink()
	if err != nil {
		return cid.Undef, err
	}
	cl, ok := l.(cidlink.Link)
	if !ok {
		return cid.Undef, errors.New("unexpected link type")
	}
	return cl.Cid, nil
}
func compareStrings(a, b string) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}
func (i *Index) Close() error {
	return i.bs.Close()
}