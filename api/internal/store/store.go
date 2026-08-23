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
	ErrNotFound    = errors.New("instância não encontrada")
	ErrExists      = errors.New("já existe uma instância com esse nome")
	ErrInvalidRoot = errors.New("raiz inválida")
)

// os erros abaixo levam o que a tela precisa, e o texto do Error() e so para log

type NotFoundError struct{ Name string }

func (e *NotFoundError) Error() string        { return fmt.Sprintf("%q: %s", e.Name, ErrNotFound) }
func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

type ExistsError struct{ Name string }

func (e *ExistsError) Error() string        { return fmt.Sprintf("%q: %s", e.Name, ErrExists) }
func (e *ExistsError) Is(target error) bool { return target == ErrExists }

// Reason diz qual regra a raiz quebrou: not_absolute, create_failed, unreadable, not_dir, unwritable
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
	composeFile = "docker-compose.yml"
	envFile     = ".env"
	metaFile    = ".okdock.json"
	// instancia de quando o projeto se chamava GameDock, e a Spec e a unica copia desses campos
	legacyMetaFile = ".gamedock.json"
)

type Store struct {
	ConfigRoot string

	mu   sync.RWMutex
	root string
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
		slog.Warn("configuração do painel ilegível; seguindo na raiz de boot", "err", err)
		return s, nil
	}
	if cfg.Root != "" && cfg.Root != abs {
		if err := prepareRoot(cfg.Root); err != nil {
			slog.Warn("raiz gravada não pôde ser usada; seguindo na raiz de boot",
				"root", cfg.Root, "err", err)
			return s, nil
		}
		s.root = cfg.Root
	}
	return s, nil
}

func (s *Store) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

func (s *Store) SetRoot(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(root) {
		return &InvalidRootError{Reason: "not_absolute", Path: root}
	}
	if err := prepareRoot(abs); err != nil {
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
		// a raiz pode ter outras coisas dentro, e nome que o painel nao aceitaria nao e instancia dele
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

func (s *Store) Get(name string) (instance.Spec, error) {
	if err := instance.ValidateName(name); err != nil {
		return instance.Spec{}, err
	}
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
		return instance.Spec{}, fmt.Errorf("lendo %s de %s: %w", metaFile, name, err)
	}
	spec.Name = name
	return spec, nil
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

	meta, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, metaFile), append(meta, '\n'), 0o644); err != nil {
		return err
	}
	// a partir daqui vale o nome novo, os dois fariam a leitura depender de qual veio primeiro
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
