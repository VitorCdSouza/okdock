package dockerx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
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

func (c CLI) Up(ctx context.Context, dir string) error {
	_, err := c.run(ctx, upTimeout, c.args(dir, "up", "-d", "--remove-orphans")...)
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

func (c CLI) Pull(ctx context.Context, dir string, progress func(line string)) error {
	ctx, cancel := context.WithTimeout(ctx, upTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.bin(), c.args(dir, "pull")...)
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
