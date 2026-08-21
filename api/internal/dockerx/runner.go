package dockerx

import (
	"context"
	"io"
	"time"
)

type Container struct {
	Name     string
	Service  string
	State    string
	Status   string
	Health   string
	ExitCode int
}

type Stats struct {
	Name        string
	CPUPercent  float64
	MemoryBytes int64
	MemoryLimit int64
}

type Runner interface {
	Up(ctx context.Context, dir string) error
	Down(ctx context.Context, dir string) error
	Restart(ctx context.Context, dir string) error
	Pull(ctx context.Context, dir string, progress func(line string)) error
	PS(ctx context.Context, dir string) ([]Container, error)
	Logs(ctx context.Context, dir string, tail int, follow bool) (io.ReadCloser, error)
	Stats(ctx context.Context, names []string) ([]Stats, error)
	Version(ctx context.Context) (string, error)
}

const (
	shortTimeout = 30 * time.Second
	upTimeout    = 10 * time.Minute
	downTimeout  = 5 * time.Minute
)
