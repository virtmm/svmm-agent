package agent_test

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/propedeutica/svmm-agent/internal/agent"
	"github.com/propedeutica/svmm-proto/events"
	cloudeventsv1 "github.com/propedeutica/svmm-proto/gen/go/io/cloudevents/v1"
	agentv1 "github.com/propedeutica/svmm-proto/gen/go/svmm/agent/v1"
)

// fakeServer records the events an agent sends and can drop connections.
type fakeServer struct {
	agentv1.UnimplementedAgentServiceServer
	got      chan *cloudeventsv1.CloudEvent
	sessions atomic.Int32
	// dropAfter closes the first session after this many events (0 = never).
	dropAfter int
}

func (s *fakeServer) Connect(stream grpc.BidiStreamingServer[cloudeventsv1.CloudEvent, cloudeventsv1.CloudEvent]) error {
	n := s.sessions.Add(1)
	count := 0
	for {
		ev, err := stream.Recv()
		if err != nil {
			return nil
		}
		s.got <- ev
		count++
		if n == 1 && s.dropAfter > 0 && count >= s.dropAfter {
			return nil // server ends the stream
		}
	}
}

func start(t *testing.T, srv *fakeServer) agentv1.AgentServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return agentv1.NewAgentServiceClient(conn)
}

func next(t *testing.T, ch <-chan *cloudeventsv1.CloudEvent) *cloudeventsv1.CloudEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

func TestSendsHeartbeats(t *testing.T) {
	srv := &fakeServer{got: make(chan *cloudeventsv1.CloudEvent, 16)}
	a := agent.New(start(t, srv), agent.Options{AgentID: "a1", HeartbeatInterval: 20 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	for range 3 {
		ev := next(t, srv.got)
		if ev.GetSource() != "svmm-agent/a1" {
			t.Fatalf("source = %q", ev.GetSource())
		}
		hb, err := events.ParseHeartbeat(ev)
		if err != nil {
			t.Fatal(err)
		}
		if hb.GetAgentVersion() != agent.Version {
			t.Fatalf("version = %q", hb.GetAgentVersion())
		}
	}

	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
}

func TestReconnectsAfterServerDropsStream(t *testing.T) {
	srv := &fakeServer{got: make(chan *cloudeventsv1.CloudEvent, 16), dropAfter: 1}
	a := agent.New(start(t, srv), agent.Options{
		AgentID: "a1", HeartbeatInterval: time.Hour, ReconnectMin: 10 * time.Millisecond, ReconnectMax: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = a.Run(ctx) }()

	next(t, srv.got) // heartbeat on first session; server then drops it
	next(t, srv.got) // heartbeat on the reconnected session
	if srv.sessions.Load() < 2 {
		t.Fatalf("sessions = %d, want >= 2", srv.sessions.Load())
	}
}
