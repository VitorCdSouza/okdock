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

type HostContainer struct {
	ID       string
	Name     string
	Image    string
	State    string
	Status   string
	Health   string
	ExitCode int
	Project  string
	Service  string
	WorkDir  string
	Labels   map[string]string
	Networks []string
	Ports    []HostPort
}

type HostPort struct {
	Host      int
	Container int
	Protocol  string
}

type Stats struct {
	Name        string
	CPUPercent  float64
	MemoryBytes int64
	MemoryLimit int64
}

type Runner interface {
	Up(ctx context.Context, dir string, services ...string) error
	Down(ctx context.Context, dir string) error
	Restart(ctx context.Context, dir string) error
	Pull(ctx context.Context, dir string, progress func(line string), services ...string) error
	PS(ctx context.Context, dir string) ([]Container, error)
	PSAll(ctx context.Context) ([]HostContainer, error)
	ContainerAction(ctx context.Context, name, verb string) error
	Logs(ctx context.Context, dir string, tail int, follow bool) (io.ReadCloser, error)
	ContainerLogs(ctx context.Context, name string, tail int, follow bool) (io.ReadCloser, error)
	Stats(ctx context.Context, names []string) ([]Stats, error)
	ImageID(ctx context.Context, ref string) (string, error)
	Version(ctx context.Context) (string, error)
}

const (
	shortTimeout = 30 * time.Second
	upTimeout    = 10 * time.Minute
	downTimeout  = 5 * time.Minute
)
