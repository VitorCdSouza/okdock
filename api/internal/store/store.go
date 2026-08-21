package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/VitorCdSouza/gamedock/api/internal/compose"
	"github.com/VitorCdSouza/gamedock/api/internal/instance"
)

var (
	ErrNotFound = errors.New("instância não encontrada")
	ErrExists   = errors.New("já existe uma instância com esse nome")
)

const (
	composeFile = "docker-compose.yml"
	envFile     = ".env"
	metaFile    = ".gamedock.json"
)

type Store struct {
	Root string
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("criando raiz %s: %w", abs, err)
	}
	return &Store{Root: abs}, nil
}

func (s *Store) Dir(name string) string {
	return filepath.Join(s.Root, name)
}

func (s *Store) ComposePath(name string) string {
	return filepath.Join(s.Dir(name), composeFile)
}

func (s *Store) List() ([]instance.Spec, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, err
	}
	var out []instance.Spec
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
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
		return instance.Spec{}, fmt.Errorf("%q: %w", name, ErrNotFound)
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
		return fmt.Errorf("%q: %w", spec.Name, ErrExists)
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
		return fmt.Errorf("%q: %w", spec.Name, ErrNotFound)
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
		return fmt.Errorf("%q: %w", name, ErrNotFound)
	}
	if !keepData {
		return os.RemoveAll(s.Dir(name))
	}
	dir := s.Dir(name)
	for _, f := range []string{composeFile, envFile, metaFile} {
		if err := os.Remove(filepath.Join(dir, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) ReadCompose(name string) ([]byte, error) {
	raw, err := os.ReadFile(s.ComposePath(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%q: %w", name, ErrNotFound)
	}
	return raw, err
}
