package api

import (
	"context"
	dkvv1 "distributed-kv/gen/dkv/v1"
	"distributed-kv/gen/dkv/v1/dkvv1connect"
	"distributed-kv/internal/store/distributed"

	"connectrpc.com/connect"
	"github.com/hashicorp/raft"
)

var _ dkvv1connect.MembershipAPIHandler = (*MembershipAPIHandler)(nil)

type MembershipAPIHandler struct {
	AdvertiseNodes map[raft.ServerID]string
	Store          *distributed.Store
}

func (m *MembershipAPIHandler) GetServers(
	context.Context,
	*connect.Request[dkvv1.GetServersRequest],
) (*connect.Response[dkvv1.GetServersResponse], error) {
	srvs, err := m.Store.GetServers()
	if err != nil {
		return nil, err
	}
	protoServers := make([]*dkvv1.Server, 0, len(srvs))
	leaderAddr, leaderID := m.Store.GetLeader()
	for _, node := range srvs {
		protoServers = append(protoServers, &dkvv1.Server{
			Id:          string(node.ID),
			RaftAddress: string(node.Address),
			RpcAddress:  m.AdvertiseNodes[node.ID],
			IsLeader:    node.ID == leaderID && node.Address == leaderAddr,
		})
	}

	return &connect.Response[dkvv1.GetServersResponse]{
		Msg: &dkvv1.GetServersResponse{
			Servers: protoServers,
		},
	}, nil
}

func (m *MembershipAPIHandler) JoinServer(
	_ context.Context,
	req *connect.Request[dkvv1.JoinServerRequest],
) (*connect.Response[dkvv1.JoinServerResponse], error) {
	return &connect.Response[dkvv1.JoinServerResponse]{}, m.Store.Join(
		raft.ServerID(req.Msg.GetId()),
		raft.ServerAddress(req.Msg.GetAddress()),
	)
}

func (m *MembershipAPIHandler) LeaveServer(
	_ context.Context,
	req *connect.Request[dkvv1.LeaveServerRequest],
) (*connect.Response[dkvv1.LeaveServerResponse], error) {
	return &connect.Response[dkvv1.LeaveServerResponse]{}, m.Store.Leave(
		raft.ServerID(req.Msg.GetId()),
	)
}
