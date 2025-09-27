package blockstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	s "ues-lite/datastore"

	"github.com/ipfs/boxo/blockservice"
	bstor "github.com/ipfs/boxo/blockstore"
	chunker "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/filestore"
	"github.com/ipfs/boxo/ipld/merkledag"
	unixfile "github.com/ipfs/boxo/ipld/unixfs/file"
	imp "github.com/ipfs/boxo/ipld/unixfs/importer"
	h "github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	trickle "github.com/ipfs/boxo/ipld/unixfs/importer/trickle"
	ufsio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/storage/bsrvadapter"
	traversal "github.com/ipld/go-ipld-prime/traversal"
	selector "github.com/ipld/go-ipld-prime/traversal/selector"
	selb "github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/multiformats/go-multihash"
)

const (
	DefaultChunkSize = 262144
	RabinMinSize     = DefaultChunkSize / 2
	RabinMaxSize     = DefaultChunkSize * 2
)

var DefaultLP = cidlink.LinkPrototype{
	Prefix: cid.Prefix{
		Version:  1,
		Codec:    uint64(cid.DagCBOR),
		MhType:   uint64(multihash.BLAKE3),
		MhLength: -1,
	},
}

type Blockstore interface {
	bstor.Blockstore
	bstor.Viewer
	io.Closer
	Datastore() s.Datastore
	//
	PutNode(ctx context.Context, n datamodel.Node) (cid.Cid, error)
	PutFile(ctx context.Context, data io.Reader, useRabin bool) (cid.Cid, error)
	AddFile(ctx context.Context, filePath string) (cid.Cid, error)
	//
	GetNode(ctx context.Context, c cid.Cid) (datamodel.Node, error)
	GetFile(ctx context.Context, c cid.Cid) (files.Node, error)
	GetReader(ctx context.Context, c cid.Cid) (io.ReadSeekCloser, error)
	Walk(ctx context.Context, root cid.Cid, visit func(p traversal.Progress, n datamodel.Node) error) error
	GetSubgraph(ctx context.Context, root cid.Cid, selectorNode datamodel.Node) ([]cid.Cid, error)
	Prefetch(ctx context.Context, root cid.Cid, selectorNode datamodel.Node, workers int) error
	ExportCARV2(ctx context.Context, root cid.Cid, selectorNode datamodel.Node, w io.Writer, opts ...carv2.WriteOption) error
	ImportCARV2(ctx context.Context, r io.Reader, opts ...carv2.ReadOption) ([]cid.Cid, error)
}

type blockstore struct {
	bstor.Blockstore
	ds   s.Datastore
	lsys *linking.LinkSystem
	bS   blockservice.BlockService
	dS   format.DAGService
	fs   *filestore.Filestore
	fm   *filestore.FileManager
}

var _ Blockstore = (*blockstore)(nil)

func NewBlockstore(ds s.Datastore, fileRoot string) *blockstore {

	absPath, err := filepath.Abs(fileRoot)
	if err != nil {
		log.Fatal("Ошибка получения абсолютного пути:", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		panic(fmt.Sprintf("не удалось создать корневой каталог для filestore: %v", err))
	}

	base := bstor.NewBlockstore(ds)

	fm := filestore.NewFileManager(ds, absPath, func(fm *filestore.FileManager) {
		fm.AllowFiles = true
	})

	bs := &blockstore{
		Blockstore: base,
		ds:         ds,
	}

	bs.fs = filestore.NewFilestore(bs, fm)
	bs.fm = fm

	bs.bS = blockservice.New(bs.fs, nil)

	bs.dS = merkledag.NewDAGService(bs.bS)

	adapter := &bsrvadapter.Adapter{Wrapped: bs.bS}
	lS := cidlink.DefaultLinkSystem()
	lS.SetWriteStorage(adapter)
	lS.SetReadStorage(adapter)
	bs.lsys = &lS

	bs.fm = fm

	return bs
}

func (bs *blockstore) PutNode(ctx context.Context, n datamodel.Node) (cid.Cid, error) {
	if bs.lsys == nil {
		return cid.Undef, errors.New("links system is nil")
	}
	lnk, err := bs.lsys.Store(ipld.LinkContext{Ctx: ctx}, DefaultLP, n)
	if err != nil {
		return cid.Undef, err
	}
	c := lnk.(cidlink.Link).Cid
	return c, nil
}

func (bs *blockstore) GetNode(ctx context.Context, c cid.Cid) (datamodel.Node, error) {
	if bs.lsys == nil {
		return nil, errors.New("link system is nil")
	}
	lnk := cidlink.Link{Cid: c}
	return bs.lsys.Load(ipld.LinkContext{Ctx: ctx}, lnk, basicnode.Prototype.Any)
}

func (bs *blockstore) PutFile(ctx context.Context, data io.Reader, useRabin bool) (cid.Cid, error) {
	var spl chunker.Splitter
	if useRabin {
		spl = chunker.NewRabinMinMax(data, RabinMinSize, DefaultChunkSize, RabinMaxSize)
	} else {
		spl = chunker.NewSizeSplitter(data, DefaultChunkSize)
	}
	nd, err := imp.BuildDagFromReader(bs.dS, spl)
	if err != nil {
		return cid.Undef, err
	}
	return nd.Cid(), nil
}

func (bs *blockstore) AddFile(ctx context.Context, filePath string) (cid.Cid, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось открыть файл: %v", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось получить информацию о файле: %v", err)
	}
	f, err := files.NewReaderPathFile(filePath, file, stat)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось создать files.File: %v", err)
	}
	defer f.Close()
	spl := chunker.NewSizeSplitter(f, DefaultChunkSize)
	dbp := h.DagBuilderParams{
		Dagserv:   bs.dS,
		Maxlinks:  h.DefaultLinksPerBlock,
		NoCopy:    true,
		RawLeaves: true,
	}
	dbh, err := dbp.New(spl)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось создать DagBuilder: %v", err)
	}
	n, err := trickle.Layout(dbh)
	if err != nil {
		return cid.Undef, fmt.Errorf("не удалось создать DAG: %v", err)
	}
	fmt.Printf("Файл '%s' (размер: %d байт) успешно добавлен в IPFS\n", filepath.Base(filePath), stat.Size())
	return n.Cid(), nil
}

func (bs *blockstore) GetFile(ctx context.Context, c cid.Cid) (files.Node, error) {
	nd, err := bs.dS.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return unixfile.NewUnixfsFile(ctx, bs.dS, nd)
}

func (bs *blockstore) GetReader(ctx context.Context, c cid.Cid) (io.ReadSeekCloser, error) {
	nd, err := bs.dS.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return ufsio.NewDagReader(ctx, nd, bs.dS)
}

func (bs *blockstore) ListAll(ctx context.Context) (func(context.Context) *filestore.ListRes, error) {
	return filestore.ListAll(ctx, bs.fs, false)
}

func (bs *blockstore) VerifyAll(ctx context.Context) (func(context.Context) *filestore.ListRes, error) {
	return filestore.VerifyAll(ctx, bs.fs, false)
}

func (bs *blockstore) List(ctx context.Context, key cid.Cid) *filestore.ListRes {
	return filestore.List(ctx, bs.fs, key)
}

func (bs *blockstore) Verify(ctx context.Context, key cid.Cid) *filestore.ListRes {
	return filestore.Verify(ctx, bs.fs, key)
}

func (bs *blockstore) View(ctx context.Context, id cid.Cid, callback func([]byte) error) error {
	if v, ok := bs.Blockstore.(bstor.Viewer); ok {
		return v.View(ctx, id, callback)
	}
	blk, err := bs.Blockstore.Get(ctx, id)
	if err != nil {
		return err
	}
	return callback(blk.RawData())
}

func buildSelectorNodeExploreAll() datamodel.Node {
	sb := selb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	return sb.ExploreRecursive(
		selector.RecursionLimitNone(),
		sb.ExploreAll(
			sb.ExploreRecursiveEdge(),
		),
	).Node()
}

func BuildSelectorNodeExploreAll() datamodel.Node {
	sb := selb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	return sb.ExploreRecursive(
		selector.RecursionLimitDepth(50),
		sb.ExploreUnion(
			sb.ExploreAll(sb.ExploreRecursiveEdge()), // Все поля/индексы
			sb.Matcher(),                             // Текущий узел
		),
	).Node()
}

func BuildSelectorNodeSimple() datamodel.Node {
	sb := selb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	// Простой селектор - выбрать текущий узел
	return sb.Matcher().Node()
}

func BuildSelectorNodeExploreFields() datamodel.Node {
	sb := selb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	return sb.ExploreAll(sb.Matcher()).Node()
}

func BuildSelectorNodeExploreIPFS() datamodel.Node {
	sb := selb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	return sb.ExploreRecursive(
		selector.RecursionLimitDepth(50),
		sb.ExploreFields(func(efsb selb.ExploreFieldsSpecBuilder) {
			// Исследуем все поля
			efsb.Insert("Links", sb.ExploreAll(
				sb.ExploreFields(func(linkBuilder selb.ExploreFieldsSpecBuilder) {
					linkBuilder.Insert("Hash", sb.ExploreRecursiveEdge())
				}),
			))
			efsb.Insert("Data", sb.Matcher())
		}),
	).Node()
}

func _BuildSelectorNodeExploreIPFS() datamodel.Node {
	sb := selb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	return sb.ExploreRecursive(
		selector.RecursionLimitDepth(100), // Разумное ограничение
		sb.ExploreUnion(
			// Исследуем поле Links
			sb.ExploreFields(func(efsb selb.ExploreFieldsSpecBuilder) {
				efsb.Insert("Links", sb.ExploreAll(
					sb.ExploreFields(func(linkFieldsBuilder selb.ExploreFieldsSpecBuilder) {
						// В каждом элементе Links ищем поле Hash (ссылка)
						linkFieldsBuilder.Insert("Hash", sb.ExploreRecursiveEdge())
					}),
				))
				// Также можем обработать Data
				efsb.Insert("Data", sb.Matcher())
			}),
			// Matcher для текущего узла
			sb.Matcher(),
		),
	).Node()
}

func (bs *blockstore) Walk(ctx context.Context, root cid.Cid, visit func(p traversal.Progress, n datamodel.Node) error) error {
	if bs.lsys == nil {
		return errors.New("link system is nil")
	}
	start, err := bs.lsys.Load(ipld.LinkContext{Ctx: ctx}, cidlink.Link{Cid: root}, basicnode.Prototype.Any)
	if err != nil {
		return fmt.Errorf("failed to load root node: %w", err)
	}
	var spec datamodel.Node
	if start.Kind() == datamodel.Kind_Map {
		if _, err := start.LookupByString("Links"); err == nil {
			// UnixFS DAG
			spec = BuildSelectorNodeExploreIPFS()
		} else {
			// Обычный CBOR/JSON DAG
			spec = BuildSelectorNodeExploreAll()
		}
	} else {
		// Raw или список
		spec = BuildSelectorNodeSimple()
	}
	sel, err := selector.CompileSelector(spec)
	if err != nil {
		return fmt.Errorf("failed to compile selector: %w", err)
	}
	cfg := traversal.Config{
		Ctx:        ctx,
		LinkSystem: *bs.lsys,
		LinkTargetNodePrototypeChooser: func(ipld.Link, ipld.LinkContext) (datamodel.NodePrototype, error) {
			return basicnode.Prototype.Any, nil
		},
	}
	prog := traversal.Progress{Cfg: &cfg}
	return prog.WalkMatching(start, sel, visit)
}

func (bs *blockstore) Close() error {
	return nil
}

func BuildSelectorExploreAll() (selector.Selector, error) {
	ssb := selb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	spec := ssb.ExploreRecursive(selector.RecursionLimitNone(),
		ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
	).Node()
	return selector.CompileSelector(spec)
}

func (bs *blockstore) GetSubgraph(ctx context.Context, root cid.Cid, selectorNode datamodel.Node) ([]cid.Cid, error) {
	start, err := bs.lsys.Load(ipld.LinkContext{Ctx: ctx}, cidlink.Link{Cid: root}, basicnode.Prototype.Any)
	if err != nil {
		return nil, err
	}
	sel, err := selector.CompileSelector(selectorNode)
	if err != nil {
		return nil, err
	}
	cfg := traversal.Config{
		LinkSystem: *bs.lsys,
		LinkTargetNodePrototypeChooser: func(ipld.Link, ipld.LinkContext) (datamodel.NodePrototype, error) {
			return basicnode.Prototype.Any, nil
		},
	}
	out := make([]cid.Cid, 0, 1024)
	out = append(out, root)
	err = traversal.Progress{Cfg: &cfg}.WalkMatching(start, sel, func(p traversal.Progress, n datamodel.Node) error {
		if p.LastBlock.Link != nil {
			if cl, ok := p.LastBlock.Link.(cidlink.Link); ok {
				out = append(out, cl.Cid)
			}
		}
		return nil
	})
	return out, err
}

func (bs *blockstore) Prefetch(ctx context.Context, root cid.Cid, selectorNode datamodel.Node, workers int) error {
	if workers <= 0 {
		workers = 8
	}
	cids, err := bs.GetSubgraph(ctx, root, selectorNode)
	if err != nil {
		return err
	}
	jobs := make(chan cid.Cid, workers*2)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for c := range jobs {
				_, _ = bs.Get(ctx, c)
			}
		}()
	}
	for _, c := range cids {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- c:
		}
	}
	close(jobs)
	wg.Wait()
	return ctx.Err()
}

func (bs *blockstore) ExportCARV2(ctx context.Context, root cid.Cid, selectorNode datamodel.Node, w io.Writer, opts ...carv2.WriteOption) error {
	if bs.lsys == nil {
		return errors.New("link system is nil")
	}
	writer, err := carv2.NewSelectiveWriter(ctx, bs.lsys, root, selectorNode, opts...)
	if err != nil {
		return err
	}
	_, err = writer.WriteTo(w)
	return err
}

func (bs *blockstore) ImportCARV2(ctx context.Context, r io.Reader, opts ...carv2.ReadOption) ([]cid.Cid, error) {
	br, err := carv2.NewBlockReader(r, opts...)
	if err != nil {
		return nil, err
	}
	roots := br.Roots
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			blk, err := br.Next()
			if err == io.EOF {
				return roots, nil
			}
			if err != nil {
				return nil, err
			}
			if err := bs.Put(ctx, blk); err != nil {
				return nil, err
			}
		}
	}
}

func (bs *blockstore) Datastore() s.Datastore {
	return bs.ds
}
