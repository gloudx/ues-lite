package blockstore

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	bindnode "github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
)

func PutStruct[T any](ctx context.Context, bs *blockstore, v *T, ts *schema.TypeSystem, typ schema.Type, lp cidlink.LinkPrototype) (cid.Cid, error) {
	n := bindnode.Wrap(v, typ)
	return bs.PutNode(ctx, n)
}

func GetStruct[T any](bs *blockstore, ctx context.Context, c cid.Cid, ts *schema.TypeSystem, typ schema.Type) (*T, error) {
	if bs.lsys == nil {
		return nil, errors.New("link system is nil")
	}
	var out *T
	var ok bool
	lnk := cidlink.Link{Cid: c}
	n, err := bs.lsys.Load(ipld.LinkContext{Ctx: ctx}, lnk, bindnode.Prototype(out, typ))
	if err != nil {
		return nil, err
	}
	w := bindnode.Unwrap(n)
	out, ok = w.(*T)
	if !ok {
		return nil, errors.New("bindnode: type assertion failed")
	}
	return out, nil
}
