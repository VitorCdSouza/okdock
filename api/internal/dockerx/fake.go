package dockerx

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Fake struct {
	mu sync.Mutex

	Containers     map[string][]Container
	HostList       []HostContainer
	LogText        map[string]string
	StatsByName    map[string]Stats
	FailUp         map[string]error
	FailPull       map[string]error
	FailDown       map[string]error
	FailAction     map[string]error
	ServerVersion  string
	ImageIDs       map[string]string
	PulledImageIDs map[string]string

	Calls []string
}

func NewFake() *Fake {
	return &Fake{
		Containers:     map[string][]Container{},
		LogText:        map[string]string{},
		StatsByName:    map[string]Stats{},
		FailUp:         map[string]error{},
		FailPull:       map[string]error{},
		FailDown:       map[string]error{},
		FailAction:     map[string]error{},
		ImageIDs:       map[string]string{},
		PulledImageIDs: map[string]string{},
		ServerVersion:  "27.1.0",
	}
}

func (f *Fake) record(op, dir string) {
	f.Calls = append(f.Calls, op+":"+dir)
}

func (f *Fake) Up(_ context.Context, dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("up", dir)
	if err := f.FailUp[dir]; err != nil {
		return err
	}
	name := nameFromDir(dir)
	f.Containers[dir] = []Container{{
		Name: name, Service: name, State: "running", Status: "Up 1 second", Health: "",
	}}
	return nil
}

func (f *Fake) Down(_ context.Context, dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("down", dir)
	if err := f.FailDown[dir]; err != nil {
		return err
	}
	delete(f.Containers, dir)
	return nil
}

func (f *Fake) Restart(_ context.Context, dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("restart", dir)
	return nil
}

func (f *Fake) Pull(_ context.Context, dir string, progress func(string)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("pull", dir)
	if progress != nil {
		progress("Pulling " + nameFromDir(dir))
	}
	if err := f.FailPull[dir]; err != nil {
		return err
	}
	for ref, id := range f.PulledImageIDs {
		f.ImageIDs[ref] = id
	}
	return nil
}

func (f *Fake) PS(_ context.Context, dir string) ([]Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ps", dir)
	return f.Containers[dir], nil
}

func (f *Fake) Logs(_ context.Context, dir string, _ int, _ bool) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("logs", dir)
	return io.NopCloser(strings.NewReader(f.LogText[dir])), nil
}

func (f *Fake) Stats(_ context.Context, names []string) ([]Stats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Stats
	for _, n := range names {
		if s, ok := f.StatsByName[n]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *Fake) ImageID(_ context.Context, ref string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ImageIDs[ref], nil
}

func (f *Fake) Version(_ context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ServerVersion == "" {
		return "", fmt.Errorf("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")
	}
	return f.ServerVersion, nil
}

func nameFromDir(dir string) string {
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		return dir[i+1:]
	}
	return dir
}

func (f *Fake) PSAll(_ context.Context) ([]HostContainer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ps-all", "")
	out := make([]HostContainer, len(f.HostList))
	copy(out, f.HostList)
	return out, nil
}

func (f *Fake) ContainerAction(_ context.Context, name, verb string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("container-"+verb, name)
	if err := f.FailAction[name]; err != nil {
		return err
	}
	for i, c := range f.HostList {
		if c.Name != name {
			continue
		}
		switch verb {
		case "stop":
			f.HostList[i].State = "exited"
			f.HostList[i].Status = "Exited (0) 1 second ago"
		default:
			f.HostList[i].State = "running"
			f.HostList[i].Status = "Up 1 second"
		}
		return nil
	}
	return fmt.Errorf("container %q does not exist", name)
}

func (f *Fake) ContainerLogs(_ context.Context, name string, _ int, _ bool) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("container-logs", name)
	return io.NopCloser(strings.NewReader(f.LogText[name])), nil
}
