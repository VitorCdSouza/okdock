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
	if len(tags) != 2 || tags[0] != "10.9.11" || tags[1] != "latest" {
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
