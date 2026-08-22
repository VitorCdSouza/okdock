package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VitorCdSouza/gamedock/api/internal/dockerx"
	"github.com/VitorCdSouza/gamedock/api/internal/duckdns"
	"github.com/VitorCdSouza/gamedock/api/internal/httpapi"
	"github.com/VitorCdSouza/gamedock/api/internal/instance"
	"github.com/VitorCdSouza/gamedock/api/internal/manager"
	"github.com/VitorCdSouza/gamedock/api/internal/store"
	"github.com/VitorCdSouza/gamedock/api/internal/system"
	"github.com/VitorCdSouza/gamedock/api/internal/webui"
)

func main() {
	if err := run(); err != nil {
		slog.Error("gamedock parou", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr        = flag.String("addr", env("GAMEDOCK_ADDR", ":8080"), "endereço de escuta")
		root        = flag.String("root", env("GAMEDOCK_ROOT", "/srv/games"), "raiz dos diretórios de instância")
		reserveFlag = flag.String("memory-reserve", env("GAMEDOCK_MEMORY_RESERVE", "2g"), "RAM reservada ao host, fora do orçamento das instâncias")
		allowOrigin = flag.String("allow-origin", env("GAMEDOCK_ALLOW_ORIGIN", ""), "origem liberada por CORS; use http://localhost:4200 com o ng serve")
		dockerBin   = flag.String("docker-bin", env("GAMEDOCK_DOCKER_BIN", "docker"), "executável do docker")
		logLevel    = flag.String("log-level", env("GAMEDOCK_LOG_LEVEL", "info"), "debug, info, warn ou error")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLevel(*logLevel),
	})))

	reserve, err := instance.ParseMemory(*reserveFlag)
	if err != nil {
		return fmt.Errorf("-memory-reserve: %w", err)
	}

	st, err := store.New(*root)
	if err != nil {
		return err
	}

	docker := dockerx.CLI{Bin: *dockerBin}
	mgr := manager.New(manager.Options{
		Store:         st,
		Docker:        docker,
		System:        &system.ProcReader{},
		DNS:           duckdns.HTTP{},
		MemoryReserve: reserve,
	})

	dnsCtx, stopDNS := context.WithCancel(context.Background())
	defer stopDNS()
	go mgr.SyncDNSEvery(dnsCtx, manager.SyncInterval)

	var web fs.FS
	if webui.Placeholder() {
		slog.Warn("frontend não embutido; só a API responde. Rode `make build` para embutir")
	}
	web = webui.FS()

	srv := &http.Server{
		Addr: *addr,
		Handler: httpapi.New(httpapi.Options{
			Manager:     mgr,
			WebFS:       web,
			AllowOrigin: *allowOrigin,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if v, err := docker.Version(ctx); err != nil {
		slog.Warn("docker não respondeu; monte /var/run/docker.sock no container", "err", err)
	} else {
		slog.Info("docker encontrado", "version", v)
	}
	cancel()

	slog.Info("gamedock ouvindo", "addr", *addr, "root", st.Root(),
		"reserva_ram", instance.FormatMemory(reserve))

	errc := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errc:
		return err
	case <-stop:
		slog.Info("desligando")
	}

	shutdownCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	return srv.Shutdown(shutdownCtx)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}
