package registry_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VitorCdSouza/okdock/api/internal/registry"
)

func TestHubPath(t *testing.T) {
	cases := []struct {
		repo string
		want string
		err  bool
	}{
		{repo: "nginx", want: "library/nginx"},
		{repo: "jellyfin/jellyfin", want: "jellyfin/jellyfin"},
		{repo: "jellyfin/jellyfin:10.9", want: "jellyfin/jellyfin"},
		{repo: "nginx:alpine", want: "library/nginx"},
		// another registry does not answer the Hub API
		{repo: "ghcr.io/vitorcdsouza/okdock", err: true},
		{repo: "lscr.io/linuxserver/jellyfin", err: true},
		{repo: "registro.local:5000/terraria", err: true},
		{repo: "", err: true},
	}
	for _, c := range cases {
		got, err := registry.HubPath(c.repo)
		if c.err {
			if !errors.Is(err, registry.ErrNotHub) {
				t.Errorf("%q: err = %v, want ErrNotHub", c.repo, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%q = %q, %v; want %q", c.repo, got, err, c.want)
		}
	}
}

func TestHubTags(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.String()
		_, _ = w.Write([]byte(`{"count":3,"results":[
			{"name":"10.9.11"},{"name":"latest"},{"name":""}]}`))
	}))
	defer srv.Close()

	tags, err := registry.Hub{Endpoint: srv.URL, Client: srv.Client()}.
		Tags(t.Context(), "jellyfin/jellyfin:10.8")
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	// latest is the tag docker assumes, so it leads the list
	if len(tags) != 2 || tags[0] != "latest" || tags[1] != "10.9.11" {
		t.Fatalf("tags = %v", tags)
	}
	if !strings.HasPrefix(asked, "/jellyfin/jellyfin/tags?") {
		t.Errorf("asked %q, the tag already typed is not part of the path", asked)
	}
	if !strings.Contains(asked, "ordering=last_updated") {
		t.Errorf("asked %q, newest first is the useful order", asked)
	}
}

func TestHubTagsOfAnImageThatIsNotThere(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tags, err := registry.Hub{Endpoint: srv.URL, Client: srv.Client()}.Tags(t.Context(), "nobody/nothing")
	if err != nil {
		t.Fatalf("a repository that does not exist is an empty list, not an error: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("tags = %v", tags)
	}
}

func TestHubTagsWhenTheRegistryIsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := registry.Hub{Endpoint: srv.URL, Client: srv.Client()}.Tags(t.Context(), "nginx")
	if !errors.Is(err, registry.ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
}

func TestHubTagsAsksForLatestWhenThePageBuriedIt(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/tags/latest") {
			_, _ = w.Write([]byte(`{"name":"latest"}`))
			return
		}
		// a nightly a day pushes latest out of the newest page
		_, _ = w.Write([]byte(`{"results":[{"name":"2026081705"},{"name":"unstable"}]}`))
	}))
	defer srv.Close()

	tags, err := registry.Hub{Endpoint: srv.URL, Client: srv.Client()}.Tags(t.Context(), "jellyfin/jellyfin")
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(tags) != 3 || tags[0] != "latest" {
		t.Fatalf("tags = %v", tags)
	}
	if len(paths) != 2 || !strings.HasSuffix(paths[1], "/tags/latest") {
		t.Fatalf("paths = %v, the second call asks for latest by name", paths)
	}
}

func TestHubTagsWithoutALatestToOffer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/latest") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"name":"1.2.3"}]}`))
	}))
	defer srv.Close()

	tags, err := registry.Hub{Endpoint: srv.URL, Client: srv.Client()}.Tags(t.Context(), "somebody/app")
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	// a tag that does not exist is not invented
	if len(tags) != 1 || tags[0] != "1.2.3" {
		t.Fatalf("tags = %v", tags)
	}
}

// the four requests of an anonymous pull: token, index, manifest of one platform and config blob
func TestHubImageConfigWalksTheManifest(t *testing.T) {
	var asked []string
	var accept []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		accept = append(accept, r.Header.Get("Accept"))
		switch {
		case strings.HasPrefix(r.URL.Path, "/token"):
			_, _ = w.Write([]byte(`{"token":"abc"}`))
		case strings.HasSuffix(r.URL.Path, "/manifests/10.9"):
			if r.Header.Get("Authorization") != "Bearer abc" {
				t.Errorf("the manifest was asked without the token")
			}
			_, _ = w.Write([]byte(`{"manifests":[
				{"digest":"sha256:arm","platform":{"os":"linux","architecture":"arm64"}},
				{"digest":"sha256:amd","platform":{"os":"linux","architecture":"amd64"}},
				{"digest":"sha256:win","platform":{"os":"windows","architecture":"amd64"}}]}`))
		case strings.HasSuffix(r.URL.Path, "/manifests/sha256:amd"):
			_, _ = w.Write([]byte(`{"config":{"digest":"sha256:cfg"}}`))
		case strings.HasSuffix(r.URL.Path, "/blobs/sha256:cfg"):
			_, _ = w.Write([]byte(`{"config":{
				"ExposedPorts":{"8096/tcp":{}},
				"Volumes":{"/config":{},"/cache":{}}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	hub := registry.Hub{Registry: srv.URL, Auth: srv.URL + "/token", Client: srv.Client()}
	cfg, err := hub.ImageConfig(t.Context(), "jellyfin/jellyfin:10.9")
	if err != nil {
		t.Fatalf("ImageConfig: %v", err)
	}
	if _, ok := cfg.ExposedPorts["8096/tcp"]; !ok {
		t.Errorf("ports = %v", cfg.ExposedPorts)
	}
	if len(cfg.Volumes) != 2 {
		t.Errorf("volumes = %v", cfg.Volumes)
	}
	if len(asked) != 4 {
		t.Fatalf("asked = %v", asked)
	}
	// amd64 wins over the arm entry that came first
	if asked[2] != "/v2/jellyfin/jellyfin/manifests/sha256:amd" {
		t.Errorf("picked the wrong platform: %v", asked)
	}
	if !strings.Contains(accept[1], "manifest.list.v2+json") {
		t.Errorf("the index was asked without accepting an index: %q", accept[1])
	}
}

func TestHubImageConfigOfAnImageWithNoTag(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		switch {
		case strings.HasPrefix(r.URL.Path, "/token"):
			_, _ = w.Write([]byte(`{"token":"abc"}`))
		case strings.HasSuffix(r.URL.Path, "/manifests/latest"):
			_, _ = w.Write([]byte(`{"config":{"digest":"sha256:cfg"}}`))
		default:
			_, _ = w.Write([]byte(`{"config":{"ExposedPorts":{"80/tcp":{}}}}`))
		}
	}))
	defer srv.Close()

	hub := registry.Hub{Registry: srv.URL, Auth: srv.URL + "/token", Client: srv.Client()}
	cfg, err := hub.ImageConfig(t.Context(), "nginx")
	if err != nil {
		t.Fatalf("ImageConfig: %v", err)
	}
	if _, ok := cfg.ExposedPorts["80/tcp"]; !ok {
		t.Errorf("ports = %v", cfg.ExposedPorts)
	}
	// a bare name is a library image, and no tag means latest
	if asked[1] != "/v2/library/nginx/manifests/latest" {
		t.Fatalf("asked = %v", asked)
	}
}
