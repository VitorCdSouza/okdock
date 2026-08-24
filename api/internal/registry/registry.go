package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultEndpoint = "https://hub.docker.com/v2/repositories"
	defaultRegistry = "https://registry-1.docker.io"
	defaultAuth     = "https://auth.docker.io/token"
)

var (
	// docker search reports no tags, so the tag list comes from the Hub and only for what it hosts
	ErrNotHub      = errors.New("the tag list only works for a Docker Hub image")
	ErrUnreachable = errors.New("could not reach the registry")
)

type UnreachableError struct{ Detail string }

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("%s: %s", ErrUnreachable, e.Detail)
}

func (e *UnreachableError) Is(target error) bool { return target == ErrUnreachable }

type Client interface {
	Tags(ctx context.Context, repo string) ([]string, error)
	ImageConfig(ctx context.Context, ref string) (Config, error)
}

type Hub struct {
	Endpoint string
	Registry string
	Auth     string
	Client   *http.Client
	Limit    int
}

// HubPath turns a reference into namespace/name, and a first element with a host is not the Hub
func HubPath(repo string) (string, error) {
	repo = strings.TrimSpace(strings.TrimSuffix(repo, "/"))
	if repo == "" {
		return "", ErrNotHub
	}
	if i := strings.LastIndex(repo, ":"); i > strings.LastIndex(repo, "/") {
		repo = repo[:i]
	}
	parts := strings.Split(repo, "/")
	switch len(parts) {
	case 1:
		return "library/" + parts[0], nil
	case 2:
		if strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost" {
			return "", ErrNotHub
		}
		return repo, nil
	default:
		return "", ErrNotHub
	}
}

type tagPage struct {
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
}

func (h Hub) Tags(ctx context.Context, repo string) ([]string, error) {
	path, err := HubPath(repo)
	if err != nil {
		return nil, err
	}
	endpoint := h.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	limit := h.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	// newest first, which is the order somebody picking a tag wants
	target := fmt.Sprintf("%s/%s/tags?page_size=%d&ordering=last_updated",
		strings.TrimSuffix(endpoint, "/"), path, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &UnreachableError{Detail: unwrapURL(err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &UnreachableError{Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	var page tagPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, &UnreachableError{Detail: err.Error()}
	}
	tags := make([]string, 0, len(page.Results))
	for _, r := range page.Results {
		if r.Name != "" {
			tags = append(tags, r.Name)
		}
	}
	return h.withLatest(ctx, client, endpoint, path, tags), nil
}

const latest = "latest"

// a nightly buries latest past the newest page, so it is asked by name and always comes first
func (h Hub) withLatest(ctx context.Context, client *http.Client, endpoint, path string, tags []string) []string {
	for i, tag := range tags {
		if tag != latest {
			continue
		}
		tags = append(tags[:i], tags[i+1:]...)
		return append([]string{latest}, tags...)
	}
	if !h.hasLatest(ctx, client, endpoint, path) {
		return tags
	}
	return append([]string{latest}, tags...)
}

func (h Hub) hasLatest(ctx context.Context, client *http.Client, endpoint, path string) bool {
	target := fmt.Sprintf("%s/%s/tags/%s", strings.TrimSuffix(endpoint, "/"), path, latest)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func unwrapURL(err error) string {
	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Err != nil {
		return uerr.Err.Error()
	}
	return err.Error()
}

type Fake struct {
	mu sync.Mutex

	TagsByRepo  map[string][]string
	ConfigByRef map[string]Config
	Err         error
	ErrOnConfig error
	Calls       []string
}

func NewFake() *Fake {
	return &Fake{TagsByRepo: map[string][]string{}, ConfigByRef: map[string]Config{}}
}

func (f *Fake) ImageConfig(_ context.Context, ref string) (Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, "config:"+ref)
	if f.ErrOnConfig != nil {
		return Config{}, f.ErrOnConfig
	}
	if _, err := HubPath(ref); err != nil {
		return Config{}, err
	}
	return f.ConfigByRef[ref], nil
}

func (f *Fake) Tags(_ context.Context, repo string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, repo)
	if f.Err != nil {
		return nil, f.Err
	}
	if _, err := HubPath(repo); err != nil {
		return nil, err
	}
	tags := f.TagsByRepo[repo]
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

// ImageConfig reads the config blob from the registry, four anonymous requests at worst
func (h Hub) ImageConfig(ctx context.Context, ref string) (Config, error) {
	path, err := HubPath(ref)
	if err != nil {
		return Config{}, err
	}
	tag := "latest"
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		tag = ref[i+1:]
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	base := h.Registry
	if base == "" {
		base = defaultRegistry
	}

	token, err := h.token(ctx, client, path)
	if err != nil {
		return Config{}, err
	}
	digest, err := h.configDigest(ctx, client, base, token, path, tag)
	if err != nil {
		return Config{}, err
	}

	var blob struct {
		Config Config `json:"config"`
	}
	if err := h.get(ctx, client, base+"/v2/"+path+"/blobs/"+digest, token, "", &blob); err != nil {
		return Config{}, err
	}
	return blob.Config, nil
}

type Config struct {
	ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	Volumes      map[string]struct{} `json:"Volumes"`
}

const acceptManifests = "application/vnd.oci.image.index.v1+json," +
	"application/vnd.docker.distribution.manifest.list.v2+json," +
	"application/vnd.oci.image.manifest.v1+json," +
	"application/vnd.docker.distribution.manifest.v2+json"

type manifest struct {
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Manifests []struct {
		Digest   string `json:"digest"`
		Platform struct {
			OS           string `json:"os"`
			Architecture string `json:"architecture"`
		} `json:"platform"`
	} `json:"manifests"`
}

// a multi platform tag answers an index, and one platform config carries ports and volumes
func (h Hub) configDigest(ctx context.Context, client *http.Client, base, token, path, tag string) (string, error) {
	var m manifest
	if err := h.get(ctx, client, base+"/v2/"+path+"/manifests/"+tag, token, acceptManifests, &m); err != nil {
		return "", err
	}
	if m.Config.Digest != "" {
		return m.Config.Digest, nil
	}
	pick := ""
	for _, entry := range m.Manifests {
		if entry.Platform.OS != "linux" || entry.Digest == "" {
			continue
		}
		if entry.Platform.Architecture == "amd64" {
			pick = entry.Digest
			break
		}
		if pick == "" {
			pick = entry.Digest
		}
	}
	if pick == "" {
		return "", &UnreachableError{Detail: "no linux image in the manifest"}
	}
	var inner manifest
	if err := h.get(ctx, client, base+"/v2/"+path+"/manifests/"+pick, token, acceptManifests, &inner); err != nil {
		return "", err
	}
	if inner.Config.Digest == "" {
		return "", &UnreachableError{Detail: "the manifest carries no config"}
	}
	return inner.Config.Digest, nil
}

func (h Hub) token(ctx context.Context, client *http.Client, path string) (string, error) {
	auth := h.Auth
	if auth == "" {
		auth = defaultAuth
	}
	var body struct {
		Token string `json:"token"`
	}
	url := fmt.Sprintf("%s?service=registry.docker.io&scope=repository:%s:pull", auth, path)
	if err := h.get(ctx, client, url, "", "", &body); err != nil {
		return "", err
	}
	return body.Token, nil
}

func (h Hub) get(ctx context.Context, client *http.Client, url, token, accept string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := client.Do(req)
	if err != nil {
		return &UnreachableError{Detail: unwrapURL(err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &UnreachableError{Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return json.NewDecoder(resp.Body).Decode(into)
}
