// Package agent runs the SVMM agent: it keeps a gRPC stream to the server and
// sends CloudEvents (currently only heartbeats) over it.
package agent

import (
	"context"
	"errors"
	"io"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/propedeutica/svmm-proto/events"
	cloudeventsv1 "github.com/propedeutica/svmm-proto/gen/go/io/cloudevents/v1"
	agentv1 "github.com/propedeutica/svmm-proto/gen/go/svmm/agent/v1"
)

// Version is the agent version reported in heartbeats. Overridden at build time:
//
//	go build -ldflags "-X github.com/propedeutica/svmm-agent/internal/agent.Version=1.2.3"
var Version = "dev"

// Options configure an Agent.
type Options struct {
	AgentID           string
	HeartbeatInterval time.Duration
	// ReconnectMin and ReconnectMax bound the exponential backoff between
	// connection attempts. Zero values use 1s and 30s.
	ReconnectMin time.Duration
	ReconnectMax time.Duration
	Logger       *zap.Logger
}

// Agent is a gRPC SVMM agent.
type Agent struct {
	client agentv1.AgentServiceClient
	opts   Options
}

// New creates an agent that talks to the server through client.
func New(client agentv1.AgentServiceClient, opts Options) *Agent {
	if opts.ReconnectMin <= 0 {
		opts.ReconnectMin = time.Second
	}
	if opts.ReconnectMax <= 0 {
		opts.ReconnectMax = 30 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Agent{client: client, opts: opts}
}

// Source is the CloudEvents source attribute of everything this agent sends.
func (a *Agent) Source() string { return "svmm-agent/" + a.opts.AgentID }

// Run connects and keeps the stream open until ctx is cancelled. A lost
// connection is retried with exponential backoff; Run never returns an error
// other than the context's.
func (a *Agent) Run(ctx context.Context) error {
	backoff := a.opts.ReconnectMin
	for {
		started := time.Now()
		err := a.session(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A session that stayed up for a while resets the backoff.
		if time.Since(started) > a.opts.ReconnectMax {
			backoff = a.opts.ReconnectMin
		}
		a.opts.Logger.Warn("connection to server lost", zap.Error(err), zap.Duration("retry_in", backoff))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, a.opts.ReconnectMax)
	}
}

// session runs one stream until it fails or ctx is cancelled.
func (a *Agent) session(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := a.client.Connect(ctx)
	if err != nil {
		return err
	}
	a.opts.Logger.Info("connected to server", zap.String("source", a.Source()))

	recvErr := make(chan error, 1)
	go func() { recvErr <- a.receive(stream) }()

	// First heartbeat immediately, then on the interval.
	ticker := time.NewTicker(a.opts.HeartbeatInterval)
	defer ticker.Stop()
	for {
		if err := a.sendHeartbeat(stream); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			_ = stream.CloseSend()
			return ctx.Err()
		case err := <-recvErr:
			return err
		case <-ticker.C:
		}
	}
}

func (a *Agent) sendHeartbeat(stream grpc.BidiStreamingClient[cloudeventsv1.CloudEvent, cloudeventsv1.CloudEvent]) error {
	ev, err := events.NewHeartbeat(a.Source(), Version, time.Now())
	if err != nil {
		return err
	}
	a.opts.Logger.Debug("sending heartbeat", zap.String("id", ev.GetId()))
	return stream.Send(ev)
}

// receive reads events from the server. Unknown types are logged and dropped so a
// newer server never breaks an older agent.
func (a *Agent) receive(stream grpc.BidiStreamingClient[cloudeventsv1.CloudEvent, cloudeventsv1.CloudEvent]) error {
	for {
		ev, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		if err != nil {
			return err
		}
		a.opts.Logger.Info("ignoring unsupported event", zap.String("type", ev.GetType()), zap.String("id", ev.GetId()))
	}
}
