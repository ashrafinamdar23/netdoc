package main

import (
	"context"
	"flag"
	"os"

	"github.com/ashrafinamdar23/netdoc/internal/config"
	"github.com/ashrafinamdar23/netdoc/internal/logx"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
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
