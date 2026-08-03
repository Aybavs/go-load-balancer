package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aybavs/go-load-balancer/internal/config"
	"github.com/aybavs/go-load-balancer/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go watchReloads(ctx, srv, *configPath)

	log.Printf("load balancer listening on %s (SIGHUP to reload)", cfg.Listen)
	if err := srv.Run(ctx); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

// watchReloads reloads the config on SIGHUP. A config that fails to load or
// validate is rejected and the running config is kept.
func watchReloads(ctx context.Context, srv *server.Server, configPath string) {
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	for {
		select {
		case <-ctx.Done():
			return
		case <-hup:
			newCfg, err := config.Load(configPath)
			if err != nil {
				log.Printf("reload: keeping current config: %v", err)
				continue
			}
			if err := srv.Reload(newCfg); err != nil {
				log.Printf("reload: keeping current config: %v", err)
				continue
			}
			log.Printf("reloaded config from %s", configPath)
		}
	}
}
