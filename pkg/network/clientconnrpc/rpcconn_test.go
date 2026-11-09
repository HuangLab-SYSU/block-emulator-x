package clientconnrpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

func TestDirectBroadcast(t *testing.T) {
	t.Run("a stalled peer does not block other sends and times out", func(t *testing.T) {
		leader := nodetopo.NodeInfo{NodeID: 0}
		healthy := nodetopo.NodeInfo{NodeID: 1}
		stalled := nodetopo.NodeInfo{NodeID: 2}
		sender := NewRPCConn(leader, nil)
		receiver := NewRPCConn(healthy, nil)
		entered := make(chan struct{})
		cancelled := make(chan struct{})

		sender.clientPool[healthy] = startTestReplica(t, receiver)
		sender.clientPool[stalled] = startTestReplica(t, receiver, grpc.UnaryInterceptor(
			func(ctx context.Context, _ any, _ *grpc.UnaryServerInfo, _ grpc.UnaryHandler) (any, error) {
				close(entered)
				<-ctx.Done()
				close(cancelled)
				return nil, ctx.Err()
			},
		))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		conn := network.NewConnHandler(sender)
		proposal := &rpcserver.WrappedMsg{MsgType: "Preprepare", Payload: []byte("migration proposal")}
		stalledDone := make(chan struct{})
		go func() {
			conn.GroupBroadcastMessage(ctx, []nodetopo.NodeInfo{stalled}, proposal)
			close(stalledDone)
		}()

		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("stalled peer did not receive the request")
		}

		healthyDone := make(chan struct{})
		go func() {
			conn.GroupBroadcastMessage(ctx, []nodetopo.NodeInfo{leader, healthy}, proposal)
			close(healthyDone)
		}()

		select {
		case <-healthyDone:
		case <-time.After(3 * time.Second):
			t.Fatal("healthy sends waited on the stalled peer")
		}

		for _, peer := range []*RPCConn{sender, receiver} {
			msgs := peer.DrainMsgBuffer()
			require.Len(t, msgs, 1)
			require.Equal(t, proposal.GetPayload(), msgs[0].GetPayload())
		}

		select {
		case <-stalledDone:
		case <-time.After(rpcTimeout + 2*time.Second):
			t.Fatal("broadcast did not return after the RPC deadline")
		}

		select {
		case <-cancelled:
		case <-time.After(2 * time.Second):
			t.Fatal("RPC was not cancelled at the peer")
		}
	})
}

func startTestReplica(t *testing.T, receiver rpcserver.ReplicaConnServer, opts ...grpc.ServerOption) *clientConnection {
	t.Helper()

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(opts...)
	rpcserver.RegisterReplicaConnServer(server, receiver)
	t.Cleanup(server.Stop)

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Error(err)
		}
	}()

	conn, err := grpc.NewClient("passthrough:///replica",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	return &clientConnection{conn: conn, client: rpcserver.NewReplicaConnClient(conn)}
}
