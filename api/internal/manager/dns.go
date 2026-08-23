package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/VitorCdSouza/okdock/api/internal/duckdns"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/store"
)

const SyncInterval = 5 * time.Minute

var (
	ErrDNSDisabled = errors.New("este painel foi iniciado sem cliente de DNS")
	ErrNoToken     = errors.New("o token do duckdns ainda não foi configurado")
	ErrDNSTaken    = errors.New("esse domínio já está vinculado a outra instância")
)

// DNSTakenError diz qual nome e qual instancia, para a tela montar a frase.
type DNSTakenError struct {
	Hostname string
	Instance string
}

func (e *DNSTakenError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", e.Hostname, ErrDNSTaken, e.Instance)
}

func (e *DNSTakenError) Is(target error) bool { return target == ErrDNSTaken }

type DNSStatus struct {
	Token   string         `json:"token"`
	Suffix  string         `json:"suffix"`
	Links   []DNSLink      `json:"links"`
	Domains []instance.DNS `json:"domains"`
}

type DNSLink struct {
	Instance string `json:"instance"`
	instance.DNS
}

func (m *Manager) DNS() DNSStatus {
	m.dnsMu.Lock()
	cfg := m.dnsCfg
	m.dnsMu.Unlock()

	out := DNSStatus{Token: cfg.Token, Suffix: duckdns.Suffix}
	for name, link := range cfg.Links {
		if !m.store.Exists(name) {
			continue
		}
		out.Links = append(out.Links, DNSLink{Instance: name, DNS: link})
	}
	sort.Slice(out.Links, func(i, j int) bool { return out.Links[i].Instance < out.Links[j].Instance })

	for _, d := range cfg.Domains {
		out.Domains = append(out.Domains, d)
	}
	sort.Slice(out.Domains, func(i, j int) bool { return out.Domains[i].Domain < out.Domains[j].Domain })
	return out
}

func (m *Manager) AddDNSDomain(ctx context.Context, domain string) (instance.DNS, error) {
	if m.dns == nil {
		return instance.DNS{}, ErrDNSDisabled
	}
	domain, err := duckdns.Normalize(domain)
	if err != nil {
		return instance.DNS{}, err
	}

	m.dnsMu.Lock()
	token := m.dnsCfg.Token
	m.dnsMu.Unlock()
	if token == "" {
		return instance.DNS{}, ErrNoToken
	}

	entry := instance.DNS{
		Domain:   domain,
		Hostname: duckdns.Hostname(domain),
		LastSync: m.now(),
	}
	res, err := m.dns.Update(ctx, token, domain)
	if err != nil {
		return instance.DNS{}, err
	}
	entry.LastIP = res.IP

	if err := m.mutateDNS(func(cfg *store.DNSConfig) error {
		cfg.Domains[domain] = entry
		return nil
	}); err != nil {
		return instance.DNS{}, err
	}
	m.hub.Publish(Event{Type: "dns.changed"})
	return entry, nil
}

func (m *Manager) RemoveDNSDomain(domain string) error {
	domain, err := duckdns.Normalize(domain)
	if err != nil {
		return err
	}
	if err := m.mutateDNS(func(cfg *store.DNSConfig) error {
		delete(cfg.Domains, domain)
		return nil
	}); err != nil {
		return err
	}
	m.hub.Publish(Event{Type: "dns.changed"})
	return nil
}

func (m *Manager) dnsFor(name string) *instance.DNS {
	m.dnsMu.Lock()
	defer m.dnsMu.Unlock()
	link, ok := m.dnsCfg.Links[name]
	if !ok {
		return nil
	}
	return &link
}

func (m *Manager) SetDNSToken(ctx context.Context, token string) error {
	if m.dns == nil {
		return ErrDNSDisabled
	}
	if token == "" {
		return fmt.Errorf("%w", ErrNoToken)
	}
	if err := m.mutateDNS(func(cfg *store.DNSConfig) error {
		cfg.Token = token
		return nil
	}); err != nil {
		return err
	}
	m.dnsBg.Add(1)
	go func() {
		defer m.dnsBg.Done()
		m.SyncDNS(context.Background())
	}()
	m.hub.Publish(Event{Type: "dns.changed"})
	return nil
}

func (m *Manager) LinkDNS(ctx context.Context, name, domain string) (instance.DNS, error) {
	if m.dns == nil {
		return instance.DNS{}, ErrDNSDisabled
	}
	if _, err := m.store.Get(name); err != nil {
		return instance.DNS{}, m.notManaged(ctx, name, err)
	}
	domain, err := duckdns.Normalize(domain)
	if err != nil {
		return instance.DNS{}, err
	}

	m.dnsMu.Lock()
	token := m.dnsCfg.Token
	for other, link := range m.dnsCfg.Links {
		if link.Domain == domain && other != name {
			m.dnsMu.Unlock()
			return instance.DNS{}, &DNSTakenError{Hostname: duckdns.Hostname(domain), Instance: other}
		}
	}
	m.dnsMu.Unlock()

	if token == "" {
		return instance.DNS{}, ErrNoToken
	}

	res, err := m.dns.Update(ctx, token, domain)
	if err != nil {
		return instance.DNS{}, err
	}

	link := instance.DNS{
		Domain:   domain,
		Hostname: duckdns.Hostname(domain),
		LastIP:   res.IP,
		LastSync: m.now(),
	}
	if err := m.mutateDNS(func(cfg *store.DNSConfig) error {
		cfg.Links[name] = link
		cfg.Domains[domain] = instance.DNS{
			Domain:   link.Domain,
			Hostname: link.Hostname,
			LastIP:   link.LastIP,
			LastSync: link.LastSync,
		}
		return nil
	}); err != nil {
		return instance.DNS{}, err
	}
	m.hub.Publish(Event{Type: "instance.changed", Instance: name})
	return link, nil
}

func (m *Manager) UnlinkDNS(name string) error {
	if err := m.mutateDNS(func(cfg *store.DNSConfig) error {
		delete(cfg.Links, name)
		return nil
	}); err != nil {
		return err
	}
	m.hub.Publish(Event{Type: "instance.changed", Instance: name})
	return nil
}

func (m *Manager) forgetDNS(name string) {
	m.dnsMu.Lock()
	_, ok := m.dnsCfg.Links[name]
	m.dnsMu.Unlock()
	if !ok {
		return
	}
	if err := m.mutateDNS(func(cfg *store.DNSConfig) error {
		delete(cfg.Links, name)
		return nil
	}); err != nil {
		slog.Warn("não consegui limpar o vínculo de DNS", "instancia", name, "err", err)
	}
}

func (m *Manager) SyncDNS(ctx context.Context) {
	if m.dns == nil {
		return
	}
	m.dnsMu.Lock()
	token := m.dnsCfg.Token
	pending := map[string]struct{}{}
	for _, link := range m.dnsCfg.Links {
		pending[link.Domain] = struct{}{}
	}
	for domain := range m.dnsCfg.Domains {
		pending[domain] = struct{}{}
	}
	m.dnsMu.Unlock()

	if token == "" || len(pending) == 0 {
		return
	}

	type outcome struct {
		ip, failure string
	}
	results := make(map[string]outcome, len(pending))
	for domain := range pending {
		res, err := m.dns.Update(ctx, token, domain)
		if err != nil {
			results[domain] = outcome{failure: err.Error()}
			continue
		}
		results[domain] = outcome{ip: res.IP}
	}

	changed := false
	err := m.mutateDNS(func(cfg *store.DNSConfig) error {
		for name, link := range cfg.Links {
			res, ok := results[link.Domain]
			if !ok {
				continue
			}
			if link.LastIP != res.ip || link.LastError != res.failure {
				changed = true
			}
			link.LastIP, link.LastError = res.ip, res.failure
			link.LastSync = m.now()
			cfg.Links[name] = link
		}
		for domain, entry := range cfg.Domains {
			res, ok := results[domain]
			if !ok {
				continue
			}
			if entry.LastIP != res.ip || entry.LastError != res.failure {
				changed = true
			}
			entry.LastIP, entry.LastError = res.ip, res.failure
			entry.LastSync = m.now()
			cfg.Domains[domain] = entry
		}
		return nil
	})
	if err != nil {
		slog.Warn("não consegui gravar o resultado do sync de DNS", "err", err)
		return
	}
	if changed {
		m.hub.Publish(Event{Type: "dns.changed"})
	}
}

func (m *Manager) SyncDNSEvery(ctx context.Context, every time.Duration) {
	m.SyncDNS(ctx)

	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.SyncDNS(ctx)
		}
	}
}

func (m *Manager) mutateDNS(fn func(*store.DNSConfig) error) error {
	m.dnsMu.Lock()
	defer m.dnsMu.Unlock()

	next := store.DNSConfig{
		Token:   m.dnsCfg.Token,
		Links:   map[string]instance.DNS{},
		Domains: map[string]instance.DNS{},
	}
	for k, v := range m.dnsCfg.Links {
		next.Links[k] = v
	}
	for k, v := range m.dnsCfg.Domains {
		next.Domains[k] = v
	}
	if err := fn(&next); err != nil {
		return err
	}
	if err := m.store.SaveDNS(next); err != nil {
		return err
	}
	m.dnsCfg = next
	return nil
}
