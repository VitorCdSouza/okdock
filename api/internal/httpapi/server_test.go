package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/manager"
	"github.com/VitorCdSouza/okdock/api/internal/registry"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/system"
	"github.com/VitorCdSouza/okdock/api/internal/template"
)

func templates(t *testing.T) *template.Catalog {
	t.Helper()
	c, err := template.NewCatalog(t.TempDir())
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return c
}

func newServer(t *testing.T) *Server {
	t.Helper()
	s, _ := newServerWithDocker(t)
	return s
}

func newServerWithRegistry(t *testing.T) (*Server, *registry.Fake) {
	t.Helper()
	st, err := store.New(store.Config{Dir: t.TempDir(), Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	cat := templates(t)
	reg := registry.NewFake()
	mgr := manager.New(manager.Options{
		Store:     st,
		Templates: cat,
		Docker:    dockerx.NewFake(),
		Registry:  reg,
		System: system.StaticReader{Info: system.Info{
			MemoryTotal: 16 << 30, MemoryAvailable: 12 << 30, CPUCount: 8,
		}},
	})
	return New(Options{Manager: mgr, Templates: cat}), reg
}

func newServerWithDocker(t *testing.T) (*Server, *dockerx.Fake) {
	t.Helper()
	st, err := store.New(store.Config{Dir: t.TempDir(), Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	cat := templates(t)
	fake := dockerx.NewFake()
	mgr := manager.New(manager.Options{
		Store:     st,
		Templates: cat,
		Docker:    fake,
		System: system.StaticReader{Info: system.Info{
			MemoryTotal: 16 << 30, MemoryAvailable: 12 << 30, CPUCount: 8,
		}},
	})
	return New(Options{Manager: mgr, Templates: cat}), fake
}

func do(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestHealth(t *testing.T) {
	if got := do(t, newServer(t), "GET", "/api/v1/health", nil).Code; got != 200 {
		t.Errorf("status = %d", got)
	}
}

func TestListTemplates(t *testing.T) {
	w := do(t, newServer(t), "GET", "/api/v1/templates", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got struct {
		Templates  []map[string]any `json:"templates"`
		Categories []string         `json:"categories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]bool, len(got.Templates))
	for _, p := range got.Templates {
		ids[p["id"].(string)] = true
	}
	for _, want := range []string{"minecraft-java", "terraria-tshock", "custom"} {
		if !ids[want] {
			t.Errorf("catalog without %q: %v", want, ids)
		}
	}
	if len(got.Categories) == 0 {
		t.Error("the category list must come along so the screen does not repeat it")
	}
}

func TestGetTemplate(t *testing.T) {
	w := do(t, newServer(t), "GET", "/api/v1/templates/minecraft-java", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body)
	}
	var p map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if p["id"] != "minecraft-java" || p["category"] != "games" {
		t.Errorf("template = %v", p)
	}
	if p["builtin"] != true {
		t.Error("a builtin template must identify itself as one")
	}
}

func TestGetTemplateUnknown(t *testing.T) {
	if got := do(t, newServer(t), "GET", "/api/v1/templates/nao-existe", nil).Code; got != 404 {
		t.Errorf("status = %d, queria 404", got)
	}
}

func TestCreateTemplateAndUseIt(t *testing.T) {
	s := newServer(t)

	fresh := map[string]any{
		"id": "jellyfin", "name": "Jellyfin", "category": "media", "short": "JF",
		"image": "jellyfin/jellyfin:10.9", "defaultMemory": "2g", "minMemory": "512m",
		"defaultCpus": 2, "stopGraceSeconds": 30,
		"ports":   []map[string]any{{"container": 8096, "protocol": "tcp", "label": "web"}},
		"volumes": []map[string]any{{"container": "/config"}},
		"fields": []map[string]any{
			{"key": "TZ", "label": "Fuso", "type": "text", "default": "America/Sao_Paulo"},
		},
	}
	if w := do(t, s, "POST", "/api/v1/templates", fresh); w.Code != 201 {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body)
	}
	if w := do(t, s, "POST", "/api/v1/templates", fresh); w.Code != 409 {
		t.Errorf("id repetido devia dar 409, veio %d", w.Code)
	}

	w := do(t, s, "POST", "/api/v1/instances", manager.SpecRequest{
		Name:       "filmes",
		TemplateID: "jellyfin",
		Values:     map[string]string{"TZ": "UTC"},
	})
	if w.Code != 201 {
		t.Fatalf("creating an instance from the new template: status = %d, body = %s", w.Code, w.Body)
	}
	var inst map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &inst)
	if inst["templateId"] != "jellyfin" || inst["category"] != "media" {
		t.Errorf("the instance did not keep the template: %v", inst)
	}
}

func TestCreateTemplateRefusesAnInvalidOne(t *testing.T) {
	w := do(t, newServer(t), "POST", "/api/v1/templates", map[string]any{
		"id": "sem categoria", "name": "", "category": "filmes",
	})
	if w.Code != 422 {
		t.Fatalf("status = %d, queria 422; corpo = %s", w.Code, w.Body)
	}
	var body struct {
		Error    string `json:"error"`
		Problems []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"problems"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Error != "invalid_fields" || len(body.Problems) == 0 {
		t.Errorf("corpo = %s", w.Body)
	}
}

func TestSaveTemplateEditsABuiltinAndDeleteUndoesIt(t *testing.T) {
	s := newServer(t)

	original := do(t, s, "GET", "/api/v1/templates/minecraft-java", nil)
	var tmpl map[string]any
	_ = json.Unmarshal(original.Body.Bytes(), &tmpl)
	tmpl["defaultMemory"] = "8g"

	if w := do(t, s, "PUT", "/api/v1/templates/minecraft-java", tmpl); w.Code != 200 {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body)
	}
	var edited map[string]any
	_ = json.Unmarshal(do(t, s, "GET", "/api/v1/templates/minecraft-java", nil).Body.Bytes(), &edited)
	if edited["defaultMemory"] != "8g" || edited["builtin"] == true {
		t.Errorf("the edit did not stick: %v", edited)
	}

	if w := do(t, s, "DELETE", "/api/v1/templates/minecraft-java", nil); w.Code != 204 {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body)
	}
	var back map[string]any
	_ = json.Unmarshal(do(t, s, "GET", "/api/v1/templates/minecraft-java", nil).Body.Bytes(), &back)
	if back["defaultMemory"] != "4g" || back["builtin"] != true {
		t.Errorf("deleting the edit should bring the builtin back: %v", back)
	}

	if w := do(t, s, "DELETE", "/api/v1/templates/minecraft-java", nil); w.Code != 409 {
		t.Errorf("an unedited builtin template should answer 409, got %d", w.Code)
	}
}

func TestCreateAndListInstance(t *testing.T) {
	s := newServer(t)

	w := do(t, s, "POST", "/api/v1/instances", manager.SpecRequest{
		Name:       "smp",
		TemplateID: "minecraft-java",
		Values:     map[string]string{"EULA": "true"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body)
	}

	w = do(t, s, "GET", "/api/v1/instances", nil)
	var resp instancesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Instances) != 1 || resp.Instances[0].Name != "smp" {
		t.Fatalf("instances = %+v", resp.Instances)
	}
	if len(resp.States) != 7 {
		t.Errorf("states = %v", resp.States)
	}
}

func TestCreateRejectsInvalidField(t *testing.T) {
	w := do(t, newServer(t), "POST", "/api/v1/instances", manager.SpecRequest{
		Name:       "smp",
		TemplateID: "minecraft-java",
		Values:     map[string]string{"DIFFICULTY": "impossivel"},
	})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, queria 422; corpo = %s", w.Code, w.Body)
	}
	var e apiError
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if len(e.Problems) == 0 {
		t.Fatal("a 422 answer must list the offending fields")
	}
	if e.Problems[0].Field != "DIFFICULTY" || e.Problems[0].Code != "not_option" {
		t.Errorf("problema = %+v, queria DIFFICULTY/not_option", e.Problems[0])
	}
}

func TestPortConflictCarriesParams(t *testing.T) {
	s := newServer(t)
	first := manager.SpecRequest{
		Name:       "smp",
		TemplateID: "minecraft-java",
		Values:     map[string]string{"EULA": "true"},
	}
	if w := do(t, s, "POST", "/api/v1/instances", first); w.Code != http.StatusCreated {
		t.Fatalf("the first create failed: %d %s", w.Code, w.Body)
	}

	second := first
	second.Name = "other"
	second.Ports = []instance.PortBinding{{Host: 25565, Container: 25565, Protocol: "tcp"}}
	w := do(t, s, "POST", "/api/v1/instances", second)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, queria 409; corpo = %s", w.Code, w.Body)
	}

	var e apiError
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Error != "port_taken" {
		t.Fatalf("code = %q, wanted port_taken", e.Error)
	}
	if e.Params["port"] != 25565.0 || e.Params["proto"] != "tcp" || e.Params["owner"] != "smp" {
		t.Errorf("params = %v, queria porta 25565/tcp de smp", e.Params)
	}
}

func TestCreateDuplicateIsConflict(t *testing.T) {
	s := newServer(t)
	req := manager.SpecRequest{
		Name:       "smp",
		TemplateID: "minecraft-java",
		Values:     map[string]string{"EULA": "true"},
	}
	do(t, s, "POST", "/api/v1/instances", req)
	if got := do(t, s, "POST", "/api/v1/instances", req).Code; got != http.StatusConflict {
		t.Errorf("status = %d, queria 409", got)
	}
}

func TestGetMissingInstanceIs404(t *testing.T) {
	if got := do(t, newServer(t), "GET", "/api/v1/instances/nao-existe", nil).Code; got != 404 {
		t.Errorf("status = %d", got)
	}
}

func TestUnknownJSONFieldIsRejected(t *testing.T) {
	s := newServer(t)
	r := httptest.NewRequest("POST", "/api/v1/instances",
		strings.NewReader(`{"name":"smp","templateId":"minecraft-java","campoQueNaoExiste":1}`))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, queria 400", w.Code)
	}
}

func TestPreviewComposeDoesNotWriteToDisk(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "POST", "/api/v1/instances/preview-compose", manager.SpecRequest{
		Name:       "smp",
		TemplateID: "minecraft-java",
		Values:     map[string]string{"EULA": "true"},
	})
	if w.Code != 200 {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body)
	}
	var resp previewComposeResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Compose, "name: smp") {
		t.Errorf("compose gerado = %s", resp.Compose)
	}

	if got := do(t, s, "GET", "/api/v1/instances", nil); !strings.Contains(got.Body.String(), `"instances":[]`) {
		t.Errorf("preview must not have created an instance: %s", got.Body)
	}
}

func TestDeleteKeepsDataByDefault(t *testing.T) {
	s := newServer(t)
	do(t, s, "POST", "/api/v1/instances", manager.SpecRequest{
		Name:       "smp",
		TemplateID: "minecraft-java",
		Values:     map[string]string{"EULA": "true"},
	})
	world := filepath.Join(s.mgr.Store().Dir("smp"), "data", "level.dat")
	if err := os.WriteFile(world, []byte("mundo"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := do(t, s, "DELETE", "/api/v1/instances/smp", nil).Code; got != http.StatusNoContent {
		t.Fatalf("status = %d", got)
	}
	if got := do(t, s, "GET", "/api/v1/instances/smp", nil).Code; got != 404 {
		t.Errorf("the instance should be gone, status = %d", got)
	}
	if _, err := os.Stat(world); err != nil {
		t.Errorf("o mundo devia ter sido preservado: %v", err)
	}
}

func TestDeleteWithKeepDataFalseRemovesTheWorld(t *testing.T) {
	s := newServer(t)
	do(t, s, "POST", "/api/v1/instances", manager.SpecRequest{
		Name:       "smp",
		TemplateID: "minecraft-java",
		Values:     map[string]string{"EULA": "true"},
	})
	dir := s.mgr.Store().Dir("smp")
	if err := os.WriteFile(filepath.Join(dir, "data", "level.dat"), []byte("mundo"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := do(t, s, "DELETE", "/api/v1/instances/smp?keepData=false", nil).Code; got != http.StatusNoContent {
		t.Fatalf("status = %d", got)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the directory should be gone, err = %v", err)
	}
}

func TestSystemEndpoint(t *testing.T) {
	w := do(t, newServer(t), "GET", "/api/v1/system", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var info manager.SystemInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.MemoryTotal == 0 || info.MemoryBudget == 0 {
		t.Errorf("system = %+v", info)
	}
}

func TestSetRoot(t *testing.T) {
	s := newServer(t)
	newRoot := filepath.Join(t.TempDir(), "jogos")

	w := do(t, s, "PUT", "/api/v1/system/root", map[string]string{"root": newRoot})
	if w.Code != 200 {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	var info manager.SystemInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Root != newRoot {
		t.Errorf("root = %q, queria %q", info.Root, newRoot)
	}
}

func TestSetTemplatesDir(t *testing.T) {
	s := newServer(t)
	dir := filepath.Join(t.TempDir(), "templates")

	w := do(t, s, "PUT", "/api/v1/system/templates", map[string]string{"templates": dir})
	if w.Code != 200 {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	var info manager.SystemInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.TemplatesRoot != dir {
		t.Errorf("templatesRoot = %q, wanted %q", info.TemplatesRoot, dir)
	}

	// a template written now lands on the folder just chosen
	body := map[string]any{
		"id": "jellyfin", "name": "Jellyfin", "category": "media", "short": "JF",
		"image": "jellyfin/jellyfin:10.9", "defaultMemory": "2g", "minMemory": "512m",
		"defaultCpus": 2, "stopGraceSeconds": 30,
		"volumes": []map[string]any{{"container": "/config"}},
	}
	if w := do(t, s, "POST", "/api/v1/templates", body); w.Code != 201 {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, "jellyfin.json")); err != nil {
		t.Errorf("the template did not land on the new folder: %v", err)
	}
}

func TestSetTemplatesDirRelative(t *testing.T) {
	w := do(t, newServer(t), "PUT", "/api/v1/system/templates", map[string]string{"templates": "templates"})
	if w.Code != 422 {
		t.Fatalf("status = %d, wanted 422", w.Code)
	}
}

func TestSetRootRelativo(t *testing.T) {
	w := do(t, newServer(t), "PUT", "/api/v1/system/root", map[string]string{"root": "jogos"})
	if w.Code != 422 {
		t.Fatalf("status = %d, queria 422", w.Code)
	}
}

func TestCORSOnlyWhenConfigured(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "GET", "/api/v1/health", nil)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS opened with no configuration: %q", got)
	}
}

func TestSearchImages(t *testing.T) {
	s, fake := newServerWithDocker(t)
	fake.SearchHits["jellyfin"] = []dockerx.ImageHit{
		{Name: "jellyfin/jellyfin", Description: "media server", Stars: 1200},
		{Name: "linuxserver/jellyfin", Stars: 800},
	}

	w := do(t, s, "GET", "/api/v1/images?q=jellyfin", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	var got struct {
		Images []dockerx.ImageHit `json:"images"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Images) != 2 || got.Images[0].Name != "jellyfin/jellyfin" {
		t.Fatalf("images = %+v", got.Images)
	}
}

func TestSearchImagesWithNoTermAsksTheDaemonNothing(t *testing.T) {
	s, fake := newServerWithDocker(t)

	w := do(t, s, "GET", "/api/v1/images?q=%20", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if strings.Contains(strings.Join(fake.Calls, " "), "search") {
		t.Errorf("an empty term reached docker: %v", fake.Calls)
	}
	if !strings.Contains(w.Body.String(), `"images":[]`) {
		t.Errorf("body = %s, want an empty list", w.Body)
	}
}

func TestImageTags(t *testing.T) {
	s, reg := newServerWithRegistry(t)
	reg.TagsByRepo["jellyfin/jellyfin"] = []string{"10.9.11", "latest"}

	w := do(t, s, "GET", "/api/v1/images/tags?image=jellyfin%2Fjellyfin", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"10.9.11"`) {
		t.Fatalf("body = %s", w.Body)
	}
}

func TestImageTagsOutsideTheHub(t *testing.T) {
	s, _ := newServerWithRegistry(t)

	w := do(t, s, "GET", "/api/v1/images/tags?image=ghcr.io%2Fsomebody%2Fapp", nil)
	if w.Code != 409 {
		t.Fatalf("status = %d, want 409: %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "tags_not_hub") {
		t.Fatalf("body = %s", w.Body)
	}
}
