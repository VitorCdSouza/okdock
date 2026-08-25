package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func spec(name string) instance.Spec {
	return instance.Spec{
		Name:             name,
		TemplateID:       "minecraft-java",
		Category:         "games",
		Image:            "itzg/minecraft-server:java21",
		Env:              map[string]string{"EULA": "true", "SENHA": "hunter2"},
		SecretKeys:       []string{"SENHA"},
		Ports:            []instance.PortBinding{{Host: 25565, Container: 25565, Protocol: "tcp"}},
		Mounts:           []instance.Mount{{Host: "./data", Container: "/data"}},
		MemoryLimit:      "4g",
		CPUs:             2,
		Restart:          "unless-stopped",
		StopGraceSeconds: 120,
	}
}

func TestCreateWritesTheThreeFiles(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, f := range []string{"docker-compose.yml", ".env", ".okdock.json"} {
		if _, err := os.Stat(filepath.Join(s.Dir("smp"), f)); err != nil {
			t.Errorf("faltou %s: %v", f, err)
		}
	}
	if info, err := os.Stat(filepath.Join(s.Dir("smp"), "data")); err != nil || !info.IsDir() {
		t.Errorf("the data directory was not created: %v", err)
	}
}

func TestEnvFileIsNotWorldReadable(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	info, err := os.Stat(filepath.Join(s.Dir("smp"), ".env"))
	if err != nil {
		t.Fatalf("stat .env: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf(".env with permission %v, expected at most 0600", perm)
	}
}

func TestCreateRefusesToOverwrite(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	err := s.Create(spec("smp"))
	if !errors.Is(err, ErrExists) {
		t.Fatalf("esperava ErrExists, veio %v", err)
	}
}

func TestCreateRollsBackOnRenderFailure(t *testing.T) {
	s := newStore(t)
	bad := spec("smp")
	bad.Image = ""

	if err := s.Create(bad); err == nil {
		t.Fatal("esperava erro de render")
	}
	if s.Exists("smp") {
		t.Error("a half-created directory was left behind")
	}
}

func TestRoundTrip(t *testing.T) {
	s := newStore(t)
	orig := spec("smp")
	if err := s.Create(orig); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get("smp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Image != orig.Image || got.MemoryLimit != orig.MemoryLimit {
		t.Errorf("spec mudou na ida e volta: %+v", got)
	}
	if got.Env["SENHA"] != "hunter2" {
		t.Errorf("the secret did not survive .okdock.json: %v", got.Env)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt was not filled in")
	}
}

func TestUpdateKeepsCreatedAt(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, _ := s.Get("smp")

	next := spec("smp")
	next.MemoryLimit = "8g"
	if err := s.Update(next); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, _ := s.Get("smp")

	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Errorf("CreatedAt mudou no update: %v -> %v", before.CreatedAt, after.CreatedAt)
	}
	if after.MemoryLimit != "8g" {
		t.Errorf("the update did not stick: %q", after.MemoryLimit)
	}
}

func TestUpdateRemovesEnvFileWhenSecretsGone(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	next := spec("smp")
	delete(next.Env, "SENHA")
	next.SecretKeys = nil
	if err := s.Update(next); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir("smp"), ".env")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf(".env devia ter sido removido, err=%v", err)
	}
}

func TestGetMissing(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get("nao-existe"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("esperava ErrNotFound, veio %v", err)
	}
}

func TestListSkipsForeignDirectories(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.Root(), "coisa-do-usuario"), 0o755); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "smp" {
		t.Errorf("List = %v", list)
	}
}

func TestDeleteKeepData(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	world := filepath.Join(s.Dir("smp"), "data", "level.dat")
	if err := os.WriteFile(world, []byte("mundo"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete("smp", true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(world); err != nil {
		t.Errorf("keepData devia preservar o mundo: %v", err)
	}
	if _, err := s.Get("smp"); !errors.Is(err, ErrNotFound) {
		t.Errorf("the instance should be gone from the listing: %v", err)
	}
}

func TestDeleteEverything(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete("smp", false); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(s.Dir("smp")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the directory should be gone: %v", err)
	}
}

func TestNameFollowsDirectory(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.Rename(s.Dir("smp"), s.Dir("smp-fresh")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("smp-fresh")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "smp-fresh" {
		t.Errorf("Name = %q, queria smp-fresh", got.Name)
	}
}

func TestSetRootPersiste(t *testing.T) {
	boot := t.TempDir()
	newRoot := filepath.Join(t.TempDir(), "jogos")

	s, err := New(boot)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetRoot(newRoot); err != nil {
		t.Fatalf("SetRoot: %v", err)
	}
	if s.Root() != newRoot {
		t.Errorf("Root = %q, queria %q", s.Root(), newRoot)
	}

	other, err := New(boot)
	if err != nil {
		t.Fatal(err)
	}
	if other.Root() != newRoot {
		t.Errorf("depois do restart Root = %q, queria %q", other.Root(), newRoot)
	}
	if other.ConfigRoot != s.ConfigRoot {
		t.Errorf("ConfigRoot = %q, queria %q", other.ConfigRoot, s.ConfigRoot)
	}
}

func TestSetRootRefusesARelativePath(t *testing.T) {
	s := newStore(t)
	before := s.Root()
	if err := s.SetRoot("jogos"); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("queria ErrInvalidRoot, veio %v", err)
	}
	if s.Root() != before {
		t.Errorf("a raiz mudou mesmo com o erro: %q", s.Root())
	}
}

func TestAnUnusableSavedRootFallsBackToTheBootOne(t *testing.T) {
	boot := t.TempDir()
	s, err := New(boot)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SavePanel(PanelConfig{Root: "/proc/nao-da-para-criar-aqui"}); err != nil {
		t.Fatal(err)
	}

	other, err := New(boot)
	if err != nil {
		t.Fatalf("New devia subir mesmo assim: %v", err)
	}
	if other.Root() != s.ConfigRoot {
		t.Errorf("Root = %q, queria a raiz de boot %q", other.Root(), s.ConfigRoot)
	}
}

func TestGetReadsASpecWithTheOldName(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	dir := s.Dir("smp")
	if err := os.Rename(filepath.Join(dir, ".okdock.json"), filepath.Join(dir, ".gamedock.json")); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get("smp")
	if err != nil {
		t.Fatalf("Get com o arquivo antigo: %v", err)
	}
	if got.TemplateID != "minecraft-java" || len(got.SecretKeys) != 1 {
		t.Errorf("spec veio incompleta: %+v", got)
	}
}

func TestUpdateSwapsTheOldSpecForTheNewOne(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	dir := s.Dir("smp")
	if err := os.Rename(filepath.Join(dir, ".okdock.json"), filepath.Join(dir, ".gamedock.json")); err != nil {
		t.Fatal(err)
	}

	updated := spec("smp")
	updated.MemoryLimit = "6g"
	if err := s.Update(updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".gamedock.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the old file should be out of the way: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".okdock.json")); err != nil {
		t.Errorf("faltou o arquivo fresh: %v", err)
	}
}

func TestThePanelReadsConfigFromTheOldFolder(t *testing.T) {
	boot := t.TempDir()
	newRoot := filepath.Join(t.TempDir(), "jogos")
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(boot, ".gamedock"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`{"root":"` + newRoot + `"}` + "\n")
	if err := os.WriteFile(filepath.Join(boot, ".gamedock", "config.json"), cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := New(boot)
	if err != nil {
		t.Fatal(err)
	}
	if s.Root() != newRoot {
		t.Errorf("Root = %q, queria a raiz gravada em .gamedock/: %q", s.Root(), newRoot)
	}
}

func TestListSkipsAFolderNamedLikeNoInstance(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, dir := range []string{"botTelegram", "Media", "with space"} {
		if err := os.MkdirAll(filepath.Join(s.Root(), dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "smp" {
		t.Errorf("expected only the panel instance, got %+v", list)
	}
}

func TestGetReadsTheComposeFile(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// what a hand edit of the file looks like
	raw, err := os.ReadFile(s.ComposePath("smp"))
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "25565:25565/tcp", "25566:25565/tcp", 1)
	edited = strings.Replace(edited, "itzg/minecraft-server:java21", "itzg/minecraft-server:java17", 1)
	if err := os.WriteFile(s.ComposePath("smp"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get("smp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Image != "itzg/minecraft-server:java17" {
		t.Errorf("image = %q, the compose file is not the truth", got.Image)
	}
	if len(got.Ports) != 1 || got.Ports[0].Host != 25566 {
		t.Errorf("ports = %+v", got.Ports)
	}
	// the sidecar still answers for what compose cannot say
	if len(got.SecretKeys) != 1 || got.SecretKeys[0] != "SENHA" {
		t.Errorf("secret keys = %v", got.SecretKeys)
	}
	if got.Env["SENHA"] != "hunter2" {
		t.Errorf("the secret did not come back from the .env: %v", got.Env)
	}
	if got.CreatedAt.IsZero() {
		t.Error("createdAt was lost")
	}
}

func TestSidecarHasNoPassword(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Dir("smp"), ".okdock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "hunter2") {
		t.Fatalf("the password is in the world readable sidecar:\n%s", raw)
	}
	if !strings.Contains(string(raw), "SENHA") {
		t.Fatal("the secret key list is not in the sidecar")
	}
}

func TestGetFallsBackWhenTheComposeIsBroken(t *testing.T) {
	s := newStore(t)
	if err := s.Create(spec("smp")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.WriteFile(s.ComposePath("smp"), []byte("services: [!!!"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("smp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Image != "itzg/minecraft-server:java21" {
		t.Errorf("image = %q, the sidecar did not answer", got.Image)
	}
	if got.Env["EULA"] != "true" || got.Env["SENHA"] != "hunter2" {
		t.Errorf("env = %v", got.Env)
	}
}

func TestSetTemplatesDirPersists(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(t.TempDir(), "templates")

	s, err := New(boot)
	if err != nil {
		t.Fatal(err)
	}
	if s.TemplatesDir() != filepath.Join(boot, ".okdock", "templates") {
		t.Fatalf("with nothing chosen TemplatesDir = %q", s.TemplatesDir())
	}
	if err := s.SetTemplatesDir(dir); err != nil {
		t.Fatalf("SetTemplatesDir: %v", err)
	}
	if s.TemplatesDir() != dir {
		t.Errorf("TemplatesDir = %q, wanted %q", s.TemplatesDir(), dir)
	}

	other, err := New(boot)
	if err != nil {
		t.Fatal(err)
	}
	if other.TemplatesDir() != dir {
		t.Errorf("after the restart TemplatesDir = %q, wanted %q", other.TemplatesDir(), dir)
	}
	// the two folders travel apart, choosing one does not move the other
	if other.Root() != other.ConfigRoot {
		t.Errorf("Root = %q, wanted the boot root %q", other.Root(), other.ConfigRoot)
	}
}

func TestSetTemplatesDirRefusesARelativePath(t *testing.T) {
	s := newStore(t)
	before := s.TemplatesDir()
	if err := s.SetTemplatesDir("templates"); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("wanted ErrInvalidRoot, got %v", err)
	}
	if s.TemplatesDir() != before {
		t.Errorf("the folder changed even with the error: %q", s.TemplatesDir())
	}
}
