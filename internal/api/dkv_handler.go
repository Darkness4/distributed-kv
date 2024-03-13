package api

import (
	"context"
	dkvv1 "distributed-kv/gen/dkv/v1"
	"distributed-kv/gen/dkv/v1/dkvv1connect"
	"distributed-kv/internal/store"

	"connectrpc.com/connect"
)

var _ dkvv1connect.DkvAPIHandler = (*DkvAPIHandler)(nil)

type DkvAPIHandler struct {
	store store.Store
}

func NewDkvAPIHandler(store store.Store) *DkvAPIHandler {
	return &DkvAPIHandler{
		store: store,
	}
}

func (d *DkvAPIHandler) Delete(
	_ context.Context,
	req *connect.Request[dkvv1.DeleteRequest],
) (*connect.Response[dkvv1.DeleteResponse], error) {
	return &connect.Response[dkvv1.DeleteResponse]{}, d.store.Delete(req.Msg.Key)
}

func (d *DkvAPIHandler) Get(
	_ context.Context,
	req *connect.Request[dkvv1.GetRequest],
) (*connect.Response[dkvv1.GetResponse], error) {
	res, err := d.store.Get(req.Msg.Key)
	if err != nil {
		return nil, err
	}
	return &connect.Response[dkvv1.GetResponse]{Msg: &dkvv1.GetResponse{Value: res}}, nil
}

func (d *DkvAPIHandler) Set(
	_ context.Context,
	req *connect.Request[dkvv1.SetRequest],
) (*connect.Response[dkvv1.SetResponse], error) {
	return &connect.Response[dkvv1.SetResponse]{}, d.store.Set(req.Msg.Key, req.Msg.Value)
}
