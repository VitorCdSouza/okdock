// package hostfs lists the folders the panel is allowed to hand to a bind mount
package hostfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrOutside = errors.New("path outside the folders the panel can reach")

type Entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Listing struct {
	Path    string   `json:"path"`
	Parent  string   `json:"parent,omitempty"`
	Roots   []string `json:"roots"`
	Entries []Entry  `json:"entries"`
}

// Browser answers for a fixed set of roots, everything else is refused
type Browser struct {
	roots func() []string
}

func New(roots func() []string) *Browser { return &Browser{roots: roots} }

// List answers the folders inside dir, or the roots themselves when dir is empty
func (b *Browser) List(dir string) (Listing, error) {
	roots := b.allowed()
	if len(roots) == 0 {
		return Listing{}, errors.New("the panel has no folder it can reach")
	}
	if strings.TrimSpace(dir) == "" {
		dir = roots[0]
	}
	clean, err := b.resolve(dir, roots)
	if err != nil {
		return Listing{}, err
	}

	names, err := os.ReadDir(clean)
	if err != nil {
		return Listing{}, err
	}
	out := Listing{Path: clean, Roots: roots, Entries: []Entry{}}
	if parent := filepath.Dir(clean); parent != clean && inside(parent, roots) {
		out.Parent = parent
	}
	for _, e := range names {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out.Entries = append(out.Entries, Entry{Name: e.Name(), Path: filepath.Join(clean, e.Name())})
	}
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].Name < out.Entries[j].Name })
	return out, nil
}

// Mkdir creates one folder, and says nothing when it is already there
func (b *Browser) Mkdir(dir string) (string, error) {
	roots := b.allowed()
	parent, err := b.resolve(filepath.Dir(filepath.Clean(dir)), roots)
	if err != nil {
		return "", err
	}
	name := filepath.Base(filepath.Clean(dir))
	if name == "." || name == string(filepath.Separator) || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("%q is not a folder name", name)
	}
	full := filepath.Join(parent, name)
	if err := os.MkdirAll(full, 0o755); err != nil {
		return "", err
	}
	return full, nil
}

// Roots are the folders the picker starts from
func (b *Browser) Roots() []string { return b.allowed() }

// Reachable answers for a folder that came from somewhere else, the settings screen and not the picker
func (b *Browser) Reachable(dir string) error {
	roots := b.allowed()
	// nothing is mounted, which is the panel running outside a container
	if len(roots) == 0 {
		return nil
	}
	_, err := b.resolve(dir, roots)
	return err
}

// resolve refuses anything the roots do not hold, symlinks followed
func (b *Browser) resolve(dir string, roots []string) (string, error) {
	clean := filepath.Clean(dir)
	if !filepath.IsAbs(clean) {
		return "", ErrOutside
	}
	if real, err := filepath.EvalSymlinks(clean); err == nil {
		clean = real
	}
	if !inside(clean, roots) {
		return "", ErrOutside
	}
	return clean, nil
}

func (b *Browser) allowed() []string {
	seen := map[string]bool{}
	var out []string
	for _, root := range b.roots() {
		clean := filepath.Clean(strings.TrimSpace(root))
		if clean == "" || clean == "." || !filepath.IsAbs(clean) || seen[clean] {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func inside(dir string, roots []string) bool {
	for _, root := range roots {
		if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
