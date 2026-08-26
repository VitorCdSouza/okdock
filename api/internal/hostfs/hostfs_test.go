package hostfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func browserFor(t *testing.T, roots ...string) *Browser {
	t.Helper()
	return New(func() []string { return roots })
}

func TestListAnswersOnlyFolders(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"media", "games", ".okdock"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte("name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	listing, err := browserFor(t, root).List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var names []string
	for _, e := range listing.Entries {
		names = append(names, e.Name)
	}
	// a file is not a place for a bind mount, and a dotfolder is the panel own business
	if len(names) != 2 || names[0] != "games" || names[1] != "media" {
		t.Fatalf("entries = %v", names)
	}
	if listing.Parent != "" {
		t.Errorf("the root has no parent to walk up to, got %q", listing.Parent)
	}
}

func TestListRefusesAPathOutsideTheRoots(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()

	_, err := browserFor(t, root).List(other)

	if !errors.Is(err, ErrOutside) {
		t.Fatalf("err = %v, wanted ErrOutside", err)
	}
}

func TestListWalksUpToTheRootAndNoFurther(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "media")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	listing, err := browserFor(t, root).List(child)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if listing.Parent != root {
		t.Errorf("parent = %q, wanted %q", listing.Parent, root)
	}
}

func TestListWithNoPathOpensOnTheFirstRoot(t *testing.T) {
	root := t.TempDir()

	listing, err := browserFor(t, root).List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if listing.Path != root {
		t.Errorf("path = %q, wanted %q", listing.Path, root)
	}
}

func TestMkdirCreatesInsideARoot(t *testing.T) {
	root := t.TempDir()

	dir, err := browserFor(t, root).Mkdir(filepath.Join(root, "mundos"))
	if err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("the folder was not created: %v", err)
	}
}

func TestMkdirRefusesOutsideTheRoots(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()

	_, err := browserFor(t, root).Mkdir(filepath.Join(other, "mundos"))

	if !errors.Is(err, ErrOutside) {
		t.Fatalf("err = %v, wanted ErrOutside", err)
	}
}

func TestARootThatIsNotThereIsDropped(t *testing.T) {
	root := t.TempDir()

	listing, err := browserFor(t, filepath.Join(root, "gone"), root).List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(listing.Roots) != 1 || listing.Roots[0] != root {
		t.Fatalf("roots = %v", listing.Roots)
	}
}

func TestBindMountsSkipsWhatTheKernelPutsThere(t *testing.T) {
	root := t.TempDir()
	table := filepath.Join(t.TempDir(), "mountinfo")
	lines := "" +
		"25 30 0:23 / /proc rw,relatime - proc proc rw\n" +
		"26 30 0:24 / /sys rw,relatime - sysfs sysfs rw\n" +
		"27 30 0:25 / /etc/hosts rw,relatime - ext4 /dev/sda1 rw\n" +
		"28 30 0:26 / " + root + " rw,relatime - ext4 /dev/sda1 rw\n" +
		"29 30 0:27 / / rw,relatime - overlay overlay rw\n"
	if err := os.WriteFile(table, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	got := bindMountsFrom(table)

	if len(got) != 1 || got[0] != root {
		t.Fatalf("mounts = %v, wanted only %q", got, root)
	}
}

func TestReachableRefusesWhatIsNotMounted(t *testing.T) {
	root := t.TempDir()
	b := New(func() []string { return []string{root} })

	if err := b.Reachable(filepath.Join(root, "containers")); err != nil {
		t.Errorf("a folder inside the mount was refused: %v", err)
	}
	if err := b.Reachable("/srv/games"); !errors.Is(err, ErrOutside) {
		t.Errorf("Reachable = %v, wanted ErrOutside", err)
	}
}

func TestReachableAllowsEverythingWithNoMount(t *testing.T) {
	b := New(func() []string { return nil })
	if err := b.Reachable("/srv/games"); err != nil {
		t.Errorf("with nothing mounted the panel is out of a container: %v", err)
	}
}
