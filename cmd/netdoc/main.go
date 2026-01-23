package main

import (
	"context"
	"os"

	"github.com/ashrafinamdar23/netdoc/internal/config"
	"github.com/ashrafinamdar23/netdoc/internal/logx"
)

func main() {
	cfg, err := config.Load("./config.yaml")
	if err != nil {
		// fallback minimal
		panic(err)
	}

	logger, closeFn, err := logx.BuildLogger(cfg.Logging)
	if err != nil {
		panic(err)
	}
	defer func() { _ = closeFn() }()

	ctx := context.Background()
	ctx = logx.WithComponent(ctx, "boot")

	logger.InfoContext(ctx, "netdoc: boot", "app", cfg.App.Name, "pid", os.Getpid())
}
