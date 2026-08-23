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
		addr        = flag.String("addr", env("OKDOCK_ADDR", ":8080"), "endereço de escuta")
		root        = flag.String("root", env("OKDOCK_ROOT", "/srv/games"), "raiz dos diretórios de instância")
		reserveFlag = flag.String("memory-reserve", env("OKDOCK_MEMORY_RESERVE", "2g"), "RAM reservada ao host, fora do orçamento das instâncias")
		allowOrigin = flag.String("allow-origin", env("OKDOCK_ALLOW_ORIGIN", ""), "origem liberada por CORS; use http://localhost:4200 com o ng serve")
		dockerBin   = flag.String("docker-bin", env("OKDOCK_DOCKER_BIN", "docker"), "executável do docker")
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

	st, err := store.New(*root)
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
			Templates:   templates,
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

// env aceita o nome antigo (GAMEDOCK_*) de quem ainda nao atualizou o compose ou o systemd
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if legacy, ok := strings.CutPrefix(key, "OKDOCK_"); ok {
		if v := os.Getenv("GAMEDOCK_" + legacy); v != "" {
			slog.Warn("variável de ambiente com o nome antigo", "usada", "GAMEDOCK_"+legacy, "nova", key)
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
