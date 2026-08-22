package duckdns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const Suffix = ".duckdns.org"

const defaultEndpoint = "https://www.duckdns.org/update"

var (
	ErrRejected      = errors.New("o duckdns recusou: confira se o token está certo e se esse domínio é da sua conta")
	ErrUnreachable   = errors.New("não consegui falar com o duckdns.org")
	ErrInvalidDomain = errors.New("domínio inválido")
)

// UnreachableError guarda o motivo: a tela monta a frase pelo codigo, o detalhe fica no log
type UnreachableError struct{ Detail string }

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("%s: %s", ErrUnreachable, e.Detail)
}

func (e *UnreachableError) Is(target error) bool { return target == ErrUnreachable }

type Result struct {
	IP      string
	IPv6    string
	Changed bool
}

type Client interface {
	Update(ctx context.Context, token, domain string) (Result, error)
}

type HTTP struct {
	Endpoint string
	Client   *http.Client
}

func (h HTTP) Update(ctx context.Context, token, domain string) (Result, error) {
	endpoint := h.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	q := url.Values{
		"domains": {domain},
		"token":   {token},
		"ip":      {""},
		"verbose": {"true"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return Result{}, err
	}

	cl := h.Client
	if cl == nil {
		cl = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := cl.Do(req)
	if err != nil {
		return Result{}, &UnreachableError{Detail: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if err != nil {
		return Result{}, &UnreachableError{Detail: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, &UnreachableError{Detail: resp.Status}
	}
	return parse(string(body))
}

func parse(body string) (Result, error) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(body), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "OK" {
		return Result{}, ErrRejected
	}
	var res Result
	if len(lines) > 1 {
		res.IP = strings.TrimSpace(lines[1])
	}
	if len(lines) > 2 {
		res.IPv6 = strings.TrimSpace(lines[2])
	}
	if len(lines) > 3 {
		res.Changed = strings.EqualFold(strings.TrimSpace(lines[3]), "UPDATED")
	}
	return res, nil
}

var domainRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func Normalize(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.Trim(s, "/.")
	s = strings.TrimSuffix(s, Suffix)
	if !domainRE.MatchString(s) {
		return "", fmt.Errorf("%w: use só letras minúsculas, dígitos e hífen — o nome antes de %s", ErrInvalidDomain, Suffix)
	}
	return s, nil
}

func Hostname(domain string) string {
	if domain == "" {
		return ""
	}
	return domain + Suffix
}

type Fake struct {
	mu sync.Mutex

	Token   string
	Domains map[string]bool
	IP      string
	Err     error
	Calls   []string

	seen map[string]bool
}

func NewFake() *Fake {
	return &Fake{IP: "187.12.3.4", seen: map[string]bool{}}
}

func (f *Fake) Update(_ context.Context, token, domain string) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, domain)
	if f.Err != nil {
		return Result{}, f.Err
	}
	if f.Token != "" && token != f.Token {
		return Result{}, ErrRejected
	}
	if f.Domains != nil && !f.Domains[domain] {
		return Result{}, ErrRejected
	}
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	changed := !f.seen[domain]
	f.seen[domain] = true
	return Result{IP: f.IP, Changed: changed}, nil
}
