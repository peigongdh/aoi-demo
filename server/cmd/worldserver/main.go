package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"aoi-demo/server/internal/config"
	"aoi-demo/server/internal/network"
	"aoi-demo/server/internal/world"
)

func main() {
	configPath := flag.String("config", "server/config/dev.yaml", "path to yaml config")
	checkConfig := flag.Bool("check-config", false, "load config and exit")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	w, err := world.New(cfg)
	if err != nil {
		log.Fatalf("create world: %v", err)
	}
	if *checkConfig {
		fmt.Printf("config ok: listen=%s, aoi=%s, tick_rate=%d\n", cfg.Server.ListenAddr, w.AOIName(), cfg.Server.TickRate)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := network.NewServer(cfg, w)
	fmt.Printf("worldserver listening on %s/ws, aoi=%s, tick_rate=%d\n", cfg.Server.ListenAddr, w.AOIName(), cfg.Server.TickRate)

	if err := server.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}
