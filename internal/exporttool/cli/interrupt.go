package cli

import (
	"context"
	"os/signal"
	"syscall"
)

var (
	CliCtx context.Context
	Cancel context.CancelFunc
)

func init() {
	CliCtx, Cancel = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
