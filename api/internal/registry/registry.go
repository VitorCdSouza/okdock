package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultEndpoint = "https://hub.docker.com/v2/repositories"

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
}

type Hub struct {
	Endpoint string
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
	return tags, nil
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

	TagsByRepo map[string][]string
	Err        error
	Calls      []string
}

func NewFake() *Fake {
	return &Fake{TagsByRepo: map[string][]string{}}
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
