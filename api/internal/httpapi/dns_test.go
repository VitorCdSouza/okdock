package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/duckdns"
	"github.com/VitorCdSouza/okdock/api/internal/manager"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/system"
)

func newDNSServer(t *testing.T) (*Server, *duckdns.Fake) {
	t.Helper()
	st, err := store.New(store.Config{Dir: t.TempDir(), Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	fake := duckdns.NewFake()
	cat := templates(t)
	mgr := manager.New(manager.Options{
		Store:     st,
		Templates: cat,
		Docker:    dockerx.NewFake(),
		System: system.StaticReader{Info: system.Info{
			MemoryTotal: 16 << 30, MemoryAvailable: 12 << 30, CPUCount: 8,
		}},
		DNS: fake,
	})
	return New(Options{Manager: mgr, Templates: cat}), fake
}

func createForDNS(t *testing.T, s *Server, name string) {
	t.Helper()
	body := map[string]any{
		"name":       name,
		"templateId": "minecraft-java",
		"values":     map[string]string{"EULA": "true"},
	}
	if got := do(t, s, "POST", "/api/v1/instances", body).Code; got != 201 {
		t.Fatalf("criando %s: status %d", name, got)
	}
}

func TestDNSVazio(t *testing.T) {
	s, _ := newDNSServer(t)
	w := do(t, s, "GET", "/api/v1/dns", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got manager.DNSStatus
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Suffix != duckdns.Suffix {
		t.Errorf("the suffix comes from the API so the frontend does not repeat the rule: %q", got.Suffix)
	}
	if got.Links == nil {
		t.Error("links devia vir como lista vazia")
	}
}

func TestDNSFullFlow(t *testing.T) {
	s, _ := newDNSServer(t)
	createForDNS(t, s, "smp")

	if got := do(t, s, "PUT", "/api/v1/dns", map[string]string{"token": "tok"}).Code; got != 200 {
		t.Fatalf("gravando token: %d", got)
	}
	if got := do(t, s, "PUT", "/api/v1/instances/smp/dns", map[string]string{"domain": "casa"}).Code; got != 200 {
		t.Fatalf("vinculando: %d", got)
	}

	w := do(t, s, "GET", "/api/v1/instances/smp", nil)
	var inst struct {
		DNS *struct {
			Hostname string `json:"hostname"`
			LastIP   string `json:"lastIp"`
		} `json:"dns"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &inst); err != nil {
		t.Fatal(err)
	}
	if inst.DNS == nil || inst.DNS.Hostname != "casa.duckdns.org" || inst.DNS.LastIP == "" {
		t.Fatalf("instance dns: %+v", inst.DNS)
	}

	if got := do(t, s, "DELETE", "/api/v1/instances/smp/dns", nil).Code; got != 204 {
		t.Fatalf("desvinculando: %d", got)
	}
}

func TestDNSInvalidDomain(t *testing.T) {
	s, _ := newDNSServer(t)
	createForDNS(t, s, "smp")
	do(t, s, "PUT", "/api/v1/dns", map[string]string{"token": "tok"})

	w := do(t, s, "PUT", "/api/v1/instances/smp/dns", map[string]string{"domain": "not valid"})
	if w.Code != 422 {
		t.Fatalf("status = %d, queria 422", w.Code)
	}
}

func TestDNSRejected(t *testing.T) {
	s, fake := newDNSServer(t)
	createForDNS(t, s, "smp")
	fake.Token = "certo"
	do(t, s, "PUT", "/api/v1/dns", map[string]string{"token": "errado"})

	w := do(t, s, "PUT", "/api/v1/instances/smp/dns", map[string]string{"domain": "casa"})
	if w.Code != 422 {
		t.Fatalf("status = %d, queria 422", w.Code)
	}
	var body apiError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "dns_rejected" {
		t.Errorf("error = %q", body.Error)
	}
}

func TestDNSWithoutAToken(t *testing.T) {
	s, _ := newDNSServer(t)
	createForDNS(t, s, "smp")
	w := do(t, s, "PUT", "/api/v1/instances/smp/dns", map[string]string{"domain": "casa"})
	if w.Code != 409 {
		t.Fatalf("status = %d, queria 409", w.Code)
	}
}

func TestDNSUnknownInstance(t *testing.T) {
	s, _ := newDNSServer(t)
	do(t, s, "PUT", "/api/v1/dns", map[string]string{"token": "tok"})
	w := do(t, s, "PUT", "/api/v1/instances/fantasma/dns", map[string]string{"domain": "casa"})
	if w.Code != 404 {
		t.Fatalf("status = %d, queria 404", w.Code)
	}
}

func TestDNSRegisteredDomain(t *testing.T) {
	s, fake := newDNSServer(t)
	fake.Domains = map[string]bool{"casa": true}
	if got := do(t, s, "PUT", "/api/v1/dns", map[string]string{"token": "tok"}).Code; got != 200 {
		t.Fatalf("token: status %d", got)
	}

	if got := do(t, s, "POST", "/api/v1/dns/domains", map[string]string{"domain": "casa"}).Code; got != 200 {
		t.Fatalf("cadastro: status %d", got)
	}

	var status manager.DNSStatus
	if err := json.Unmarshal(do(t, s, "GET", "/api/v1/dns", nil).Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Domains) != 1 || status.Domains[0].Hostname != "casa.duckdns.org" {
		t.Fatalf("domains = %+v", status.Domains)
	}

	if got := do(t, s, "DELETE", "/api/v1/dns/domains/casa", nil).Code; got != 204 {
		t.Fatalf("removal: status %d", got)
	}
	if err := json.Unmarshal(do(t, s, "GET", "/api/v1/dns", nil).Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Domains) != 0 {
		t.Errorf("domains = %+v, queria vazio", status.Domains)
	}
}

func TestDNSRegisteredDomainRejected(t *testing.T) {
	s, fake := newDNSServer(t)
	fake.Domains = map[string]bool{"casa": true}
	if got := do(t, s, "PUT", "/api/v1/dns", map[string]string{"token": "tok"}).Code; got != 200 {
		t.Fatalf("token: status %d", got)
	}
	if got := do(t, s, "POST", "/api/v1/dns/domains", map[string]string{"domain": "de-outro"}).Code; got != 422 {
		t.Errorf("status = %d, queria 422", got)
	}
}
