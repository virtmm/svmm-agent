// Command svmm-agent is the SVMM compute-node agent.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/propedeutica/svmm-agent/internal/agent"
	"github.com/propedeutica/svmm-agent/internal/config"
	agentv1 "github.com/propedeutica/svmm-proto/gen/go/svmm/agent/v1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "svmm-agent:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromOS()
	if err != nil {
		return err
	}
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()

	creds := credentials.NewTLS(nil)
	if cfg.Insecure {
		logger.Warn("TLS disabled: development use only")
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(cfg.ServerAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := agent.New(agentv1.NewAgentServiceClient(conn), agent.Options{
		AgentID:           cfg.AgentID,
		HeartbeatInterval: cfg.HeartbeatInterval,
		Logger:            logger,
	})
	logger.Info("starting", zap.String("agent_id", cfg.AgentID), zap.String("server", cfg.ServerAddr), zap.String("version", agent.Version))
	if err := a.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
