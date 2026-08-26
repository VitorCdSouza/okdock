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
	"strings"
	"syscall"
	"time"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/duckdns"
	"github.com/VitorCdSouza/okdock/api/internal/httpapi"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/manager"
	"github.com/VitorCdSouza/okdock/api/internal/registry"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/system"
	"github.com/VitorCdSouza/okdock/api/internal/template"
	"github.com/VitorCdSouza/okdock/api/internal/webui"
)

func main() {
	if err := run(); err != nil {
		slog.Error("okdock parou", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr        = flag.String("addr", env("OKDOCK_ADDR", ":8080"), "listen address")
		root        = flag.String("root", env("OKDOCK_ROOT", ""), "instance folder to start with, when none was chosen on the settings screen")
		configDir   = flag.String("config", env("OKDOCK_CONFIG", "/config"), "folder where the panel keeps its own files")
		tmplDir     = flag.String("templates", env("OKDOCK_TEMPLATES", "/templates"), "folder for the templates written in the panel, until another one is chosen")
		reserveFlag = flag.String("memory-reserve", env("OKDOCK_MEMORY_RESERVE", "2g"), "RAM reserved for the host, outside the instance budget")
		allowOrigin = flag.String("allow-origin", env("OKDOCK_ALLOW_ORIGIN", ""), "origem liberada por CORS; use http://localhost:4200 com o ng serve")
		dockerBin   = flag.String("docker-bin", env("OKDOCK_DOCKER_BIN", "docker"), "docker executable")
		logLevel    = flag.String("log-level", env("OKDOCK_LOG_LEVEL", "info"), "debug, info, warn ou error")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLevel(*logLevel),
	})))

	reserve, err := instance.ParseMemory(*reserveFlag)
	if err != nil {
		return fmt.Errorf("-memory-reserve: %w", err)
	}

	st, err := store.New(store.Config{Dir: *configDir, Root: *root, Templates: *tmplDir})
	if err != nil {
		return err
	}

	templates, err := template.NewCatalog(st.TemplatesDir())
	if templates == nil {
		return err
	}
	if err != nil {
		slog.Warn("template ignorado", "dir", st.TemplatesDir(), "err", err)
	}

	docker := dockerx.CLI{Bin: *dockerBin}
	mgr := manager.New(manager.Options{
		Store:         st,
		Templates:     templates,
		Docker:        docker,
		System:        &system.ProcReader{},
		DNS:           duckdns.HTTP{},
		Registry:      registry.Hub{},
		MemoryReserve: reserve,
	})

	dnsCtx, stopDNS := context.WithCancel(context.Background())
	defer stopDNS()
	go mgr.SyncDNSEvery(dnsCtx, manager.SyncInterval)

	var web fs.FS
	if webui.Placeholder() {
		slog.Warn("frontend not embedded, only the API answers; run `make build` to embed it")
	}
	web = webui.FS()

	srv := &http.Server{
		Addr: *addr,
		Handler: httpapi.New(httpapi.Options{
			Manager:     mgr,
			Templates:   templates,
			WebFS:       web,
			AllowOrigin: *allowOrigin,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if v, err := docker.Version(ctx); err != nil {
		slog.Warn("docker did not answer, mount /var/run/docker.sock in the container", "err", err)
	} else {
		slog.Info("docker encontrado", "version", v)
	}
	cancel()

	slog.Info("okdock ouvindo", "addr", *addr, "root", st.Root(),
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
	if legacy, ok := strings.CutPrefix(key, "OKDOCK_"); ok {
		if v := os.Getenv("GAMEDOCK_" + legacy); v != "" {
			slog.Warn("environment variable with the old name", "used", "GAMEDOCK_"+legacy, "new", key)
			return v
		}
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
