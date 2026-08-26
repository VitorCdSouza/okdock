package manager

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/template"
)

// what the panel fills in on its own: no form fields, an image does not declare what it reads
type Suggestion struct {
	Ports   []template.Port   `json:"ports"`
	Volumes []template.Volume `json:"volumes"`
}

// SuggestFromImage asks the image, a running container and the registry, in that order
func (m *Manager) SuggestFromImage(ctx context.Context, ref string) (Suggestion, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Suggestion{}, nil
	}

	info, err := m.docker.ImageConfig(ctx, ref)
	if err != nil {
		slog.Debug("the image is not on the host, asking the registry", "image", ref, "err", err)
		info = m.fromRegistry(ctx, ref)
	}

	volumes := info.Volumes
	if len(volumes) == 0 {
		mounted, err := m.docker.ContainerVolumes(ctx, ref)
		if err != nil {
			slog.Debug("could not look at a container of this image", "image", ref, "err", err)
		}
		volumes = mounted
	}

	out := Suggestion{Ports: []template.Port{}, Volumes: []template.Volume{}}
	for _, p := range info.Ports {
		out.Ports = append(out.Ports, template.Port{
			Container: p.Container,
			Protocol:  p.Protocol,
		})
	}
	for _, dir := range volumes {
		out.Volumes = append(out.Volumes, template.Volume{Container: dir})
	}
	return out, nil
}

func (m *Manager) fromRegistry(ctx context.Context, ref string) dockerx.ImageInfo {
	if m.registry == nil {
		return dockerx.ImageInfo{}
	}
	cfg, err := m.registry.ImageConfig(ctx, ref)
	if err != nil {
		slog.Debug("the registry did not answer for the image", "image", ref, "err", err)
		return dockerx.ImageInfo{}
	}
	info := dockerx.ImageInfo{}
	for raw := range cfg.ExposedPorts {
		port, proto, ok := strings.Cut(raw, "/")
		if !ok {
			proto = "tcp"
		}
		n, err := strconv.Atoi(port)
		if err != nil {
			continue
		}
		info.Ports = append(info.Ports, dockerx.HostPort{Container: n, Host: n, Protocol: proto})
	}
	for dir := range cfg.Volumes {
		info.Volumes = append(info.Volumes, dir)
	}
	sort.Slice(info.Ports, func(i, j int) bool { return info.Ports[i].Container < info.Ports[j].Container })
	sort.Strings(info.Volumes)
	return info
}
