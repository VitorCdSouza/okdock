package dockerx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CLI struct {
	Bin string
}

func (c CLI) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "docker"
}

func (c CLI) args(dir string, rest ...string) []string {
	return append([]string{"compose", "--project-directory", dir}, rest...)
}

func (c CLI) run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), &Error{
			Args:   args,
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    err,
		}
	}
	return stdout.Bytes(), nil
}

type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("docker %s: %s", strings.Join(e.Args, " "), e.Stderr)
	}
	return fmt.Sprintf("docker %s: %v", strings.Join(e.Args, " "), e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// no service means the whole project and a swept orphan, one service touches only that one
func (c CLI) Up(ctx context.Context, dir string, services ...string) error {
	args := []string{"up", "-d"}
	if len(services) == 0 {
		args = append(args, "--remove-orphans")
	} else {
		args = append(args, "--no-deps")
		args = append(args, services...)
	}
	_, err := c.run(ctx, upTimeout, c.args(dir, args...)...)
	return err
}

func (c CLI) Down(ctx context.Context, dir string) error {
	_, err := c.run(ctx, downTimeout, c.args(dir, "down", "--remove-orphans")...)
	return err
}

func (c CLI) Restart(ctx context.Context, dir string) error {
	_, err := c.run(ctx, downTimeout, c.args(dir, "restart")...)
	return err
}

func (c CLI) Pull(ctx context.Context, dir string, progress func(line string), services ...string) error {
	ctx, cancel := context.WithTimeout(ctx, upTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.bin(), c.args(dir, append([]string{"pull"}, services...)...)...)
	pipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}

	var last strings.Builder
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		last.Reset()
		last.WriteString(line)
		if progress != nil {
			progress(line)
		}
	}
	if err := cmd.Wait(); err != nil {
		return &Error{Args: []string{"compose", "pull"}, Stderr: last.String(), Err: err}
	}
	return nil
}

type psLine struct {
	Name     string `json:"Name"`
	Service  string `json:"Service"`
	State    string `json:"State"`
	Status   string `json:"Status"`
	Health   string `json:"Health"`
	ExitCode int    `json:"ExitCode"`
}

func (c CLI) PS(ctx context.Context, dir string) ([]Container, error) {
	out, err := c.run(ctx, shortTimeout, c.args(dir, "ps", "--all", "--format", "json")...)
	if err != nil {
		return nil, err
	}
	return parsePS(out)
}

func parsePS(out []byte) ([]Container, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var lines []psLine
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &lines); err != nil {
			return nil, fmt.Errorf("lendo docker compose ps: %w", err)
		}
	} else {
		sc := bufio.NewScanner(bytes.NewReader(trimmed))
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var l psLine
			if err := json.Unmarshal(line, &l); err != nil {
				return nil, fmt.Errorf("lendo docker compose ps: %w", err)
			}
			lines = append(lines, l)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}

	out2 := make([]Container, 0, len(lines))
	for _, l := range lines {
		out2 = append(out2, Container{
			Name:     l.Name,
			Service:  l.Service,
			State:    strings.ToLower(l.State),
			Status:   l.Status,
			Health:   strings.ToLower(l.Health),
			ExitCode: l.ExitCode,
		})
	}
	return out2, nil
}

func (c CLI) Logs(ctx context.Context, dir string, tail int, follow bool) (io.ReadCloser, error) {
	args := c.args(dir, "logs", "--no-color", "--tail", strconv.Itoa(tail))
	if follow {
		args = append(args, "--follow")
	}
	return c.stream(ctx, args...)
}

func (c CLI) stream(ctx context.Context, args ...string) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &procReader{ReadCloser: pipe, cmd: cmd}, nil
}

type procReader struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (p *procReader) Close() error {
	err := p.ReadCloser.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = p.cmd.Wait()
	return err
}

type statsLine struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
}

func (c CLI) Stats(ctx context.Context, names []string) ([]Stats, error) {
	if len(names) == 0 {
		return nil, nil
	}
	args := append([]string{"stats", "--no-stream", "--format", "json"}, names...)
	out, err := c.run(ctx, shortTimeout, args...)
	if err != nil {
		return nil, nil
	}

	var res []Stats
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var l statsLine
		if json.Unmarshal(line, &l) != nil {
			continue
		}
		used, limit := parseMemUsage(l.MemUsage)
		res = append(res, Stats{
			Name:        l.Name,
			CPUPercent:  parsePercent(l.CPUPerc),
			MemoryBytes: used,
			MemoryLimit: limit,
		})
	}
	return res, nil
}

func parsePercent(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"), 64)
	if err != nil {
		return 0
	}
	return f
}

func parseMemUsage(s string) (used, limit int64) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	return parseSize(parts[0]), parseSize(parts[1])
}

func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	mults := []struct {
		suffix string
		mult   float64
	}{
		{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
		{"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1},
	}
	for _, m := range mults {
		if strings.HasSuffix(s, m.suffix) {
			n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, m.suffix)), 64)
			if err != nil {
				return 0
			}
			return int64(n * m.mult)
		}
	}
	return 0
}

func (c CLI) ImageID(ctx context.Context, ref string) (string, error) {
	out, err := c.run(ctx, shortTimeout, "image", "inspect", ref, "--format", "{{.Id}}")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (c CLI) Version(ctx context.Context) (string, error) {
	out, err := c.run(ctx, shortTimeout, "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type hostPSLine struct {
	ID       string `json:"ID"`
	Names    string `json:"Names"`
	Image    string `json:"Image"`
	State    string `json:"State"`
	Status   string `json:"Status"`
	Ports    string `json:"Ports"`
	Labels   string `json:"Labels"`
	Networks string `json:"Networks"`
}

func (c CLI) PSAll(ctx context.Context) ([]HostContainer, error) {
	out, err := c.run(ctx, shortTimeout, "ps", "--all", "--no-trunc", "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseHostPS(out)
}

func parseHostPS(out []byte) ([]HostContainer, error) {
	sc := bufio.NewScanner(bytes.NewReader(bytes.TrimSpace(out)))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var list []HostContainer
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var l hostPSLine
		if err := json.Unmarshal(line, &l); err != nil {
			return nil, fmt.Errorf("lendo docker ps: %w", err)
		}
		labels := parseLabels(l.Labels)
		health, code := parseStatus(l.Status)
		list = append(list, HostContainer{
			ID:       l.ID,
			Name:     strings.TrimSpace(strings.Split(l.Names, ",")[0]),
			Image:    l.Image,
			State:    strings.ToLower(l.State),
			Status:   l.Status,
			Health:   health,
			ExitCode: code,
			Project:  labels["com.docker.compose.project"],
			Service:  labels["com.docker.compose.service"],
			WorkDir:  labels["com.docker.compose.project.working_dir"],
			Labels:   labels,
			Networks: splitList(l.Networks),
			Ports:    parsePorts(l.Ports),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseLabels(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, seen := out[key]; seen {
			continue
		}
		out[key] = value
	}
	return out
}

var statusCode = regexp.MustCompile(`^Exited \((\d+)\)`)

func parseStatus(status string) (health string, exitCode int) {
	switch {
	case strings.Contains(status, "(healthy)"):
		health = "healthy"
	case strings.Contains(status, "(unhealthy)"):
		health = "unhealthy"
	case strings.Contains(status, "(health: starting)"):
		health = "starting"
	}
	if m := statusCode.FindStringSubmatch(status); m != nil {
		exitCode, _ = strconv.Atoi(m[1])
	}
	return health, exitCode
}

func parsePorts(raw string) []HostPort {
	var out []HostPort
	// the same port comes twice, once for IPv4 and once for IPv6
	seen := map[HostPort]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		published, target, ok := strings.Cut(part, "->")
		if !ok {
			continue
		}
		hostPort := published
		if i := strings.LastIndex(published, ":"); i >= 0 {
			hostPort = published[i+1:]
		}
		containerPort, protocol, _ := strings.Cut(target, "/")
		host, err := strconv.Atoi(hostPort)
		if err != nil {
			continue
		}
		container, err := strconv.Atoi(containerPort)
		if err != nil {
			continue
		}
		p := HostPort{Host: host, Container: container, Protocol: protocol}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (c CLI) ContainerAction(ctx context.Context, name, verb string) error {
	_, err := c.run(ctx, downTimeout, verb, name)
	return err
}

func (c CLI) ContainerLogs(ctx context.Context, name string, tail int, follow bool) (io.ReadCloser, error) {
	args := []string{"logs", "--tail", strconv.Itoa(tail)}
	if follow {
		args = append(args, "--follow")
	}
	return c.stream(ctx, append(args, name)...)
}
