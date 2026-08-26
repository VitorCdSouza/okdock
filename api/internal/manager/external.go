package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VitorCdSouza/okdock/api/internal/compose"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

// updateExternal edits one service and takes only that service back up
func (m *Manager) updateExternal(ctx context.Context, inst instance.Instance, req SpecRequest) (instance.Spec, error) {
	if !inst.Editable || inst.ComposeFile == "" {
		return instance.Spec{}, &ExternalError{Name: inst.Name}
	}
	spec, err := applyRequest(inst.Spec, req)
	if err != nil {
		return instance.Spec{}, err
	}
	if err := m.checkPorts(ctx, spec); err != nil {
		return instance.Spec{}, err
	}
	if isUp(inst.State) {
		if err := m.checkBudget(ctx, spec); err != nil {
			return instance.Spec{}, err
		}
	}
	if err := writeService(inst.ComposeFile, inst.Service, spec); err != nil {
		return instance.Spec{}, err
	}

	if err := m.beginOp(inst.Name, OpUpdate, "recreating"); err != nil {
		return spec, err
	}
	dir, service := filepath.Dir(inst.ComposeFile), inst.Service
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		m.endOp(inst.Name, m.docker.Up(c, dir, service))
	}()
	return spec, nil
}

// no template behind a container from outside, so what is left out keeps the value in the file
func applyRequest(spec instance.Spec, req SpecRequest) (instance.Spec, error) {
	if image := strings.TrimSpace(req.Image); image != "" {
		spec.Image = image
	}
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}
	// the form knows only the guessed template keys, so what it sends is merged, never the truth
	for k, v := range req.Values {
		spec.Env[k] = v
	}
	if len(req.Ports) > 0 {
		spec.Ports = req.Ports
	}
	if len(req.Mounts) > 0 {
		spec.Mounts = req.Mounts
	}
	if limit := strings.TrimSpace(req.MemoryLimit); limit != "" {
		want, err := instance.ParseMemory(limit)
		if err != nil {
			return instance.Spec{}, err
		}
		spec.MemoryLimit = instance.FormatMemory(want)
	}
	if req.CPUs > 0 {
		spec.CPUs = req.CPUs
	}
	if req.Restart != "" {
		spec.Restart = req.Restart
	}
	for i, p := range spec.Ports {
		if p.Host < 1 || p.Host > 65535 || p.Container < 1 || p.Container > 65535 {
			return instance.Spec{}, fmt.Errorf("invalid port: %s", p)
		}
		if p.Protocol != "tcp" && p.Protocol != "udp" {
			spec.Ports[i].Protocol = "tcp"
		}
	}
	return spec, nil
}

// the file belongs to someone else: reread before writing, same permissions, atomic rename
func writeService(file, service string, spec instance.Spec) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	project, err := compose.Parse(raw)
	if err != nil {
		return err
	}
	if len(project.Unsupported) > 0 {
		return fmt.Errorf("%s uses %s, which the panel does not write back",
			filepath.Base(file), strings.Join(project.Unsupported, ", "))
	}
	if err := project.Apply(service, spec); err != nil {
		return err
	}
	out, err := project.Bytes()
	if err != nil {
		return err
	}

	perm := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		perm = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), "."+filepath.Base(file)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

// pullExternal updates the image of one service without touching the others of the stack
func (m *Manager) pullExternal(inst instance.Instance) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	dir, service := filepath.Dir(inst.ComposeFile), inst.Service
	before, _ := m.docker.ImageID(ctx, inst.Image)

	err := m.docker.Pull(ctx, dir, func(line string) {
		m.progress(inst.Name, "", line, nil)
	}, service)
	if err != nil {
		m.endOp(inst.Name, err)
		return
	}
	after, _ := m.docker.ImageID(ctx, inst.Image)
	if before != "" && before == after {
		m.progress(inst.Name, "already_up_to_date", "", nil)
		m.endOp(inst.Name, nil)
		return
	}
	m.progress(inst.Name, "starting_new_config", "", nil)
	m.endOp(inst.Name, m.docker.Up(ctx, dir, service))
}
