package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VitorCdSouza/okdock/api/internal/compose"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

var (
	ErrNotFound    = errors.New("instance not found")
	ErrExists      = errors.New("an instance with that name already exists")
	ErrInvalidRoot = errors.New("invalid root")
)

type NotFoundError struct{ Name string }

func (e *NotFoundError) Error() string        { return fmt.Sprintf("%q: %s", e.Name, ErrNotFound) }
func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

type ExistsError struct{ Name string }

func (e *ExistsError) Error() string        { return fmt.Sprintf("%q: %s", e.Name, ErrExists) }
func (e *ExistsError) Is(target error) bool { return target == ErrExists }

type InvalidRootError struct {
	Reason string
	Path   string
	Detail string
}

func (e *InvalidRootError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", ErrInvalidRoot, e.Path, e.Detail)
	}
	return fmt.Sprintf("%s: %s", ErrInvalidRoot, e.Path)
}

func (e *InvalidRootError) Is(target error) bool { return target == ErrInvalidRoot }

const (
	composeFile    = "docker-compose.yml"
	envFile        = ".env"
	metaFile       = ".okdock.json"
	legacyMetaFile = ".gamedock.json"
)

type Store struct {
	ConfigRoot string

	mu        sync.RWMutex
	root      string
	templates string
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("criando raiz %s: %w", abs, err)
	}
	s := &Store{ConfigRoot: abs, root: abs}

	cfg, err := s.LoadPanel()
	if err != nil {
		slog.Warn("unreadable panel config, staying on the boot root", "err", err)
		return s, nil
	}
	if cfg.Root != "" && cfg.Root != abs {
		if err := prepareRoot(cfg.Root); err != nil {
			slog.Warn("the saved root could not be used, staying on the boot root",
				"root", cfg.Root, "err", err)
		} else {
			s.root = cfg.Root
		}
	}
	if cfg.Templates != "" {
		if err := prepareRoot(cfg.Templates); err != nil {
			slog.Warn("the saved templates folder could not be used, staying on the panel one",
				"templates", cfg.Templates, "err", err)
		} else {
			s.templates = cfg.Templates
		}
	}
	return s, nil
}

func (s *Store) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

func (s *Store) SetRoot(root string) error {
	abs, err := usableDir(root)
	if err != nil {
		return err
	}
	cfg, err := s.LoadPanel()
	if err != nil {
		cfg = PanelConfig{}
	}
	cfg.Root = abs
	if err := s.SavePanel(cfg); err != nil {
		return err
	}

	s.mu.Lock()
	s.root = abs
	s.mu.Unlock()
	return nil
}

// the templates folder travels like the instances one: on the boot root, never on itself
func (s *Store) SetTemplatesDir(dir string) error {
	abs, err := usableDir(dir)
	if err != nil {
		return err
	}
	cfg, err := s.LoadPanel()
	if err != nil {
		cfg = PanelConfig{}
	}
	cfg.Templates = abs
	if err := s.SavePanel(cfg); err != nil {
		return err
	}

	s.mu.Lock()
	s.templates = abs
	s.mu.Unlock()
	return nil
}

func usableDir(dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		return "", &InvalidRootError{Reason: "not_absolute", Path: dir}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, prepareRoot(abs)
}

func prepareRoot(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return &InvalidRootError{Reason: "create_failed", Path: root, Detail: err.Error()}
	}
	info, err := os.Stat(root)
	if err != nil {
		return &InvalidRootError{Reason: "unreadable", Path: root, Detail: err.Error()}
	}
	if !info.IsDir() {
		return &InvalidRootError{Reason: "not_dir", Path: root}
	}
	probe, err := os.MkdirTemp(root, ".okdock-probe-*")
	if err != nil {
		return &InvalidRootError{Reason: "unwritable", Path: root}
	}
	return os.Remove(probe)
}

func (s *Store) Dir(name string) string {
	return filepath.Join(s.Root(), name)
}

func (s *Store) ComposePath(name string) string {
	return filepath.Join(s.Dir(name), composeFile)
}

func (s *Store) List() ([]instance.Spec, error) {
	entries, err := os.ReadDir(s.Root())
	if err != nil {
		return nil, err
	}
	var out []instance.Spec
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if instance.ValidateName(e.Name()) != nil {
			continue
		}
		spec, err := s.Get(e.Name())
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get reads the instance from the compose and the sidecar, which answers when the compose cannot
func (s *Store) Get(name string) (instance.Spec, error) {
	if err := instance.ValidateName(name); err != nil {
		return instance.Spec{}, err
	}
	meta, err := s.meta(name)
	if err != nil {
		return instance.Spec{}, err
	}
	spec := meta
	spec.Name = name

	if svc, err := s.service(name); err != nil {
		slog.Warn("reading the compose file, falling back to the sidecar",
			"instance", name, "err", err)
	} else {
		spec = merge(meta, svc.Spec())
		spec.Name = name
	}
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}

	// the secret never goes to the compose file, it lives in the .env
	raw, err := os.ReadFile(filepath.Join(s.Dir(name), envFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return instance.Spec{}, err
	}
	for k, v := range compose.ParseEnv(raw) {
		spec.Env[k] = v
	}
	return spec, nil
}

func (s *Store) meta(name string) (instance.Spec, error) {
	raw, err := os.ReadFile(filepath.Join(s.Dir(name), metaFile))
	if errors.Is(err, os.ErrNotExist) {
		raw, err = os.ReadFile(filepath.Join(s.Dir(name), legacyMetaFile))
	}
	if errors.Is(err, os.ErrNotExist) {
		return instance.Spec{}, &NotFoundError{Name: name}
	}
	if err != nil {
		return instance.Spec{}, err
	}
	var spec instance.Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return instance.Spec{}, fmt.Errorf("reading %s of %s: %w", metaFile, name, err)
	}
	return spec, nil
}

func (s *Store) service(name string) (compose.Service, error) {
	raw, err := os.ReadFile(s.ComposePath(name))
	if err != nil {
		return compose.Service{}, err
	}
	project, err := compose.Parse(raw)
	if err != nil {
		return compose.Service{}, err
	}
	svc, ok := project.Service(name)
	if !ok {
		return compose.Service{}, fmt.Errorf("%s has no service named %s", composeFile, name)
	}
	return svc, nil
}

// the compose answers for the config, the sidecar for what the format cannot say
func merge(meta, fromFile instance.Spec) instance.Spec {
	out := fromFile
	out.SecretKeys = meta.SecretKeys
	out.Archived = meta.Archived
	out.CreatedAt, out.UpdatedAt = meta.CreatedAt, meta.UpdatedAt
	if out.TemplateID == "" {
		out.TemplateID = meta.TemplateID
	}
	if out.Category == "" {
		out.Category = meta.Category
	}
	if out.Env == nil {
		out.Env = map[string]string{}
	}

	labels := make(map[int]string, len(meta.Ports))
	for _, p := range meta.Ports {
		labels[p.Container] = p.Label
	}
	for i, p := range out.Ports {
		out.Ports[i].Label = labels[p.Container]
	}

	return out
}

func (s *Store) Exists(name string) bool {
	_, err := os.Stat(s.Dir(name))
	return err == nil
}

func (s *Store) Create(spec instance.Spec) error {
	if err := instance.ValidateName(spec.Name); err != nil {
		return err
	}
	if s.Exists(spec.Name) {
		return &ExistsError{Name: spec.Name}
	}
	now := time.Now().UTC()
	spec.CreatedAt, spec.UpdatedAt = now, now
	if err := os.MkdirAll(s.Dir(spec.Name), 0o755); err != nil {
		return err
	}
	if err := s.write(spec); err != nil {
		_ = os.RemoveAll(s.Dir(spec.Name))
		return err
	}
	return nil
}

func (s *Store) Update(spec instance.Spec) error {
	if !s.Exists(spec.Name) {
		return &NotFoundError{Name: spec.Name}
	}
	old, err := s.Get(spec.Name)
	if err != nil {
		return err
	}
	spec.CreatedAt = old.CreatedAt
	spec.UpdatedAt = time.Now().UTC()
	return s.write(spec)
}

func (s *Store) write(spec instance.Spec) error {
	dir := s.Dir(spec.Name)

	yml, err := compose.Render(spec)
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, composeFile), yml, 0o644); err != nil {
		return err
	}

	envPath := filepath.Join(dir, envFile)
	if env := compose.RenderEnv(spec); env != nil {
		if err := writeAtomic(envPath, env, 0o600); err != nil {
			return err
		}
	} else if err := os.Remove(envPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	meta, err := json.MarshalIndent(withoutSecrets(spec), "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, metaFile), append(meta, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, legacyMetaFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	for _, m := range spec.Mounts {
		if !strings.HasPrefix(m.Host, "./") {
			continue
		}
		if err := os.MkdirAll(filepath.Join(dir, filepath.Clean(m.Host)), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// the sidecar is world readable, so it keeps the key list and the .env keeps the password
func withoutSecrets(spec instance.Spec) instance.Spec {
	env := make(map[string]string, len(spec.Env))
	for k, v := range spec.Env {
		env[k] = v
	}
	for _, k := range spec.SecretKeys {
		delete(env, k)
	}
	spec.Env = env
	return spec
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (s *Store) Delete(name string, keepData bool) error {
	if err := instance.ValidateName(name); err != nil {
		return err
	}
	if !s.Exists(name) {
		return &NotFoundError{Name: name}
	}
	if !keepData {
		return os.RemoveAll(s.Dir(name))
	}
	dir := s.Dir(name)
	for _, f := range []string{composeFile, envFile, metaFile, legacyMetaFile} {
		if err := os.Remove(filepath.Join(dir, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) ReadCompose(name string) ([]byte, error) {
	raw, err := os.ReadFile(s.ComposePath(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, &NotFoundError{Name: name}
	}
	return raw, err
}
