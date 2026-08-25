package template

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

const CustomID = "custom"

//go:embed builtin/*.json
var builtinFS embed.FS

type Catalog struct {
	dir string

	mu      sync.RWMutex
	builtin map[string]Template
	user    map[string]Template
}

type NotFoundError struct{ ID string }

func (e *NotFoundError) Error() string { return fmt.Sprintf("template %q does not exist", e.ID) }

var ErrNotFound = errors.New("template does not exist")

func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

type BuiltinError struct{ ID string }

func (e *BuiltinError) Error() string { return fmt.Sprintf("template %q ships with OkDock", e.ID) }

var ErrBuiltin = errors.New("builtin template")

func (e *BuiltinError) Is(target error) bool { return target == ErrBuiltin }

func NewCatalog(dir string) (*Catalog, error) {
	c := &Catalog{dir: dir, user: map[string]Template{}}
	builtin, err := loadBuiltin()
	if err != nil {
		return nil, err
	}
	c.builtin = builtin
	return c, c.Reload()
}

func loadBuiltin() (map[string]Template, error) {
	entries, err := fs.Glob(builtinFS, "builtin/*.json")
	if err != nil {
		return nil, err
	}
	out := make(map[string]Template, len(entries))
	for _, name := range entries {
		raw, err := builtinFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var t Template
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("builtin template %s: %w", name, err)
		}
		if problems := t.Check(); len(problems) > 0 {
			return nil, fmt.Errorf("builtin template %s: %v", name, problems)
		}
		out[t.ID] = t
	}
	return out, nil
}

func (c *Catalog) Reload() error {
	user := map[string]Template{}
	entries, err := os.ReadDir(c.dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var bad []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(c.dir, e.Name()))
		if err != nil {
			bad = append(bad, e.Name())
			continue
		}
		var t Template
		if err := json.Unmarshal(raw, &t); err != nil {
			bad = append(bad, e.Name())
			continue
		}
		if t.ID != strings.TrimSuffix(e.Name(), ".json") {
			bad = append(bad, e.Name())
			continue
		}
		user[t.ID] = t
	}

	c.mu.Lock()
	c.user = user
	c.mu.Unlock()

	if len(bad) > 0 {
		return fmt.Errorf("unreadable templates, skipped: %s", strings.Join(bad, ", "))
	}
	return nil
}

func (c *Catalog) Get(id string) (Template, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if t, ok := c.user[id]; ok {
		return t, true
	}
	if t, ok := c.builtin[id]; ok {
		t.Builtin = true
		return t, true
	}
	if canon, ok := legacyIDs[id]; ok {
		if t, ok := c.user[canon]; ok {
			return t, true
		}
		if t, ok := c.builtin[canon]; ok {
			t.Builtin = true
			return t, true
		}
	}
	return Template{}, false
}

var legacyIDs = map[string]string{
	"itzg/minecraft-server":  "minecraft-java",
	"ryshe/terraria":         "terraria-tshock",
	"ryshe/terraria-vanilla": "terraria-vanilla",
}

func (c *Catalog) All() []Template {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Template, 0, len(c.builtin)+len(c.user))
	for id, t := range c.builtin {
		if _, edited := c.user[id]; edited {
			continue
		}
		t.Builtin = true
		out = append(out, t)
	}
	for _, t := range c.user {
		out = append(out, t)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].ID == CustomID) != (out[j].ID == CustomID) {
			return out[j].ID == CustomID
		}
		if ri, rj := categoryRank(out[i].Category), categoryRank(out[j].Category); ri != rj {
			return ri < rj
		}
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// the shipped categories keep their order, the ones a template invented come next, and other closes the list
func categoryRank(c Category) int {
	if c == CategoryOther {
		return len(AllCategories) + 1
	}
	for i, known := range AllCategories {
		if known == c {
			return i
		}
	}
	return len(AllCategories)
}

func (c *Catalog) Categories() []Category {
	var extra []Category
	seen := map[Category]bool{}
	for _, t := range c.All() {
		if t.Category.Builtin() || seen[t.Category] {
			continue
		}
		seen[t.Category] = true
		extra = append(extra, t.Category)
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i] < extra[j] })

	out := make([]Category, 0, len(AllCategories)+len(extra))
	for _, known := range AllCategories {
		if known != CategoryOther {
			out = append(out, known)
		}
	}
	out = append(out, extra...)
	return append(out, CategoryOther)
}

func (c *Catalog) Save(t Template) error {
	if problems := t.Check(); len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	t.Builtin = false

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(c.dir, t.ID+".json"), append(raw, '\n'), 0o644); err != nil {
		return err
	}

	c.mu.Lock()
	c.user[t.ID] = t
	c.mu.Unlock()
	return nil
}

func (c *Catalog) Delete(id string) error {
	c.mu.RLock()
	_, isUser := c.user[id]
	_, isBuiltin := c.builtin[id]
	c.mu.RUnlock()

	if !isUser {
		if isBuiltin {
			return &BuiltinError{ID: id}
		}
		return &NotFoundError{ID: id}
	}

	if err := os.Remove(filepath.Join(c.dir, id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	c.mu.Lock()
	delete(c.user, id)
	c.mu.Unlock()
	return nil
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

func (t Template) Check() []Problem {
	var problems []Problem

	if !idPattern.MatchString(t.ID) {
		problems = append(problems, Problem{Field: "id", Code: "bad_template_id", Params: map[string]any{"value": t.ID}})
	}
	if strings.TrimSpace(t.Name) == "" {
		problems = append(problems, Problem{Field: "name", Code: "required"})
	}
	if !t.Category.Valid() {
		problems = append(problems, Problem{Field: "category", Code: "unknown_category", Params: map[string]any{"value": string(t.Category)}})
	}
	if strings.TrimSpace(t.Image) == "" && t.ID != CustomID {
		problems = append(problems, Problem{Field: "image", Code: "required"})
	}
	problems = append(problems, t.checkMemory()...)
	problems = append(problems, t.checkPorts()...)
	problems = append(problems, t.checkVolumes()...)
	problems = append(problems, t.checkFields()...)
	return problems
}

func (t Template) checkMemory() []Problem {
	var problems []Problem
	for field, value := range map[string]string{"defaultMemory": t.DefaultMemory, "minMemory": t.MinMemory} {
		if value == "" {
			continue
		}
		if _, err := instance.ParseMemory(value); err != nil {
			problems = append(problems, Problem{Field: field, Code: "bad_memory", Params: map[string]any{"value": value}})
		}
	}
	sort.Slice(problems, func(i, j int) bool { return problems[i].Field < problems[j].Field })
	return problems
}

func (t Template) checkPorts() []Problem {
	var problems []Problem
	seen := map[string]bool{}
	for _, p := range t.Ports {
		if p.Container < 1 || p.Container > 65535 {
			problems = append(problems, Problem{Field: "ports", Code: "bad_port", Params: map[string]any{"value": p.Container}})
			continue
		}
		if p.Protocol != "tcp" && p.Protocol != "udp" {
			problems = append(problems, Problem{Field: "ports", Code: "bad_protocol", Params: map[string]any{"value": p.Protocol}})
			continue
		}
		key := fmt.Sprintf("%d/%s", p.Container, p.Protocol)
		if seen[key] {
			problems = append(problems, Problem{Field: "ports", Code: "duplicate_port", Params: map[string]any{"value": key}})
		}
		seen[key] = true
	}
	return problems
}

func (t Template) checkVolumes() []Problem {
	var problems []Problem
	for _, v := range t.Volumes {
		if !strings.HasPrefix(v.Container, "/") {
			problems = append(problems, Problem{Field: "volumes", Code: "container_path_not_absolute", Params: map[string]any{"value": v.Container}})
		}
	}
	return problems
}

func (t Template) checkFields() []Problem {
	var problems []Problem
	seen := map[string]bool{}
	for _, f := range t.Fields {
		if strings.TrimSpace(f.Key) == "" {
			problems = append(problems, Problem{Field: "fields", Code: "required"})
			continue
		}
		if seen[f.Key] {
			problems = append(problems, Problem{Field: f.Key, Code: "duplicate_field"})
		}
		seen[f.Key] = true

		switch f.Type {
		case FieldText, FieldPassword, FieldInt, FieldFloat, FieldBool, FieldEnum:
		default:
			problems = append(problems, Problem{Field: f.Key, Code: "bad_field_type", Params: map[string]any{"value": string(f.Type)}})
		}
		if f.Type == FieldEnum && len(f.Options) == 0 {
			problems = append(problems, Problem{Field: f.Key, Code: "enum_without_options"})
		}
		if f.Target == TargetArg && f.Flag == "" {
			problems = append(problems, Problem{Field: f.Key, Code: "arg_without_flag"})
		}
		if f.Default != "" {
			if _, bad := validateField(f, f.Default); bad != nil {
				problems = append(problems, Problem{Field: f.Key, Code: bad.Code, Params: bad.Params})
			}
		}
	}
	return problems
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
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
