package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/duckdns"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/system"
)

func newDNSManager(t *testing.T) (*Manager, *duckdns.Fake) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fake := duckdns.NewFake()
	m := New(Options{
		Store:     st,
		Templates: templates(t),
		Docker:    dockerx.NewFake(),
		System: system.StaticReader{Info: system.Info{
			MemoryTotal: 16 * gb, MemoryAvailable: 16 * gb, CPUCount: 8,
		}},
		DNS:           fake,
		MemoryReserve: 2 << 30,
	})
	t.Cleanup(m.dnsBg.Wait)
	return m, fake
}

func mustCreate(t *testing.T, m *Manager, name string) {
	t.Helper()
	if _, err := m.Create(context.Background(), req(name, "2g")); err != nil {
		t.Fatal(err)
	}
}

func TestLinkDNSVerificaEGrava(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")

	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	link, err := m.LinkDNS(context.Background(), "smp", "SMP.duckdns.org")
	if err != nil {
		t.Fatal(err)
	}
	if link.Domain != "smp" || link.Hostname != "smp.duckdns.org" {
		t.Errorf("o domínio devia sair normalizado: %+v", link)
	}
	if link.LastIP != fake.IP {
		t.Errorf("LastIP = %q, queria %q", link.LastIP, fake.IP)
	}

	inst, err := m.Get(context.Background(), "smp")
	if err != nil {
		t.Fatal(err)
	}
	if inst.DNS == nil || inst.DNS.Hostname != "smp.duckdns.org" {
		t.Fatalf("a instância devia trazer o DNS: %+v", inst.DNS)
	}
}

func TestLinkDNSSemToken(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")

	if _, err := m.LinkDNS(context.Background(), "smp", "smp"); !errors.Is(err, ErrNoToken) {
		t.Fatalf("queria ErrNoToken, veio %v", err)
	}
	if len(fake.Calls) != 0 {
		t.Error("sem token não faz sentido bater no duckdns")
	}
}

func TestLinkDNSRecusadoNaoGrava(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")
	fake.Domains = map[string]bool{"outro": true}

	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "smp"); !errors.Is(err, duckdns.ErrRejected) {
		t.Fatalf("queria ErrRejected, veio %v", err)
	}
	if len(m.DNS().Links) != 0 {
		t.Error("o vínculo não devia ter sido gravado")
	}
}

func TestLinkDNSDominioJaUsado(t *testing.T) {
	m, _ := newDNSManager(t)
	mustCreate(t, m, "smp")
	mustCreate(t, m, "criativo")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "criativo", "casa"); !errors.Is(err, ErrDNSTaken) {
		t.Fatalf("queria ErrDNSTaken, veio %v", err)
	}
}

func TestLinkDNSInstanciaInexistente(t *testing.T) {
	m, _ := newDNSManager(t)
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "fantasma", "casa"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("queria ErrNotFound, veio %v", err)
	}
}

func TestDNSPersisteEntreManagers(t *testing.T) {
	m, _ := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}

	outro := New(Options{
		Store:  m.store,
		Docker: dockerx.NewFake(),
		System: system.StaticReader{},
		DNS:    duckdns.NewFake(),
	})
	got := outro.DNS()
	if got.Token != "tok" || len(got.Links) != 1 || got.Links[0].Domain != "casa" {
		t.Fatalf("configuração não voltou do disco: %+v", got)
	}
}

func TestSyncDNSAtualizaIP(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}

	fake.IP = "200.1.2.3"
	m.SyncDNS(context.Background())

	if got := m.DNS().Links[0].LastIP; got != "200.1.2.3" {
		t.Errorf("LastIP = %q, queria o IP novo", got)
	}
}

func TestSyncDNSGuardaAFalha(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}

	fake.Err = duckdns.ErrUnreachable
	m.SyncDNS(context.Background())

	if got := m.DNS().Links[0].LastError; got == "" {
		t.Error("a falha do sync devia ficar registrada no vínculo")
	}
}

func TestSyncDNSSemTokenNaoSaiParaRede(t *testing.T) {
	m, fake := newDNSManager(t)
	m.SyncDNS(context.Background())
	if len(fake.Calls) != 0 {
		t.Error("sem token configurado o painel não pode chamar o duckdns")
	}
}

func TestDeleteEsqueceOVinculo(t *testing.T) {
	m, _ := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete(context.Background(), "smp", false); err != nil {
		t.Fatal(err)
	}
	if len(m.DNS().Links) != 0 {
		t.Error("vínculo órfão sobrou depois de apagar a instância")
	}
}

func TestUnlinkDNS(t *testing.T) {
	m, _ := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}
	if err := m.UnlinkDNS("smp"); err != nil {
		t.Fatal(err)
	}
	if len(m.DNS().Links) != 0 {
		t.Fatal("o vínculo devia ter saído")
	}
	if m.DNS().Token != "tok" {
		t.Error("o token não devia sumir junto")
	}
}

func TestDNSDesligadoSemCliente(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	if err := m.SetDNSToken(context.Background(), "tok"); !errors.Is(err, ErrDNSDisabled) {
		t.Fatalf("queria ErrDNSDisabled, veio %v", err)
	}
}

func TestAddDNSDomainVerificaEGrava(t *testing.T) {
	m, fake := newDNSManager(t)
	fake.Domains = map[string]bool{"casa": true}
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}

	entry, err := m.AddDNSDomain(context.Background(), "CASA.duckdns.org")
	if err != nil {
		t.Fatalf("AddDNSDomain: %v", err)
	}
	if entry.Domain != "casa" || entry.Hostname != "casa.duckdns.org" || entry.LastIP == "" {
		t.Errorf("entrada = %+v", entry)
	}
	if got := m.DNS().Domains; len(got) != 1 || got[0].Domain != "casa" {
		t.Errorf("Domains = %+v", got)
	}
}

func TestAddDNSDomainRecusadoNaoEntraNaLista(t *testing.T) {
	m, fake := newDNSManager(t)
	fake.Domains = map[string]bool{"casa": true}
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}

	if _, err := m.AddDNSDomain(context.Background(), "de-outro"); !errors.Is(err, duckdns.ErrRejected) {
		t.Fatalf("queria ErrRejected, veio %v", err)
	}
	if got := m.DNS().Domains; len(got) != 0 {
		t.Errorf("Domains = %+v, queria vazio", got)
	}
}

func TestAddDNSDomainSemToken(t *testing.T) {
	m, fake := newDNSManager(t)
	if _, err := m.AddDNSDomain(context.Background(), "casa"); !errors.Is(err, ErrNoToken) {
		t.Fatalf("queria ErrNoToken, veio %v", err)
	}
	if len(fake.Calls) != 0 {
		t.Error("sem token o painel não pode chamar o duckdns")
	}
}

func TestRemoveDNSDomain(t *testing.T) {
	m, _ := newDNSManager(t)
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AddDNSDomain(context.Background(), "casa"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveDNSDomain("casa"); err != nil {
		t.Fatalf("RemoveDNSDomain: %v", err)
	}
	if got := m.DNS().Domains; len(got) != 0 {
		t.Errorf("Domains = %+v, queria vazio", got)
	}
}

func TestLinkDNSCadastraONome(t *testing.T) {
	m, _ := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}
	if got := m.DNS().Domains; len(got) != 1 || got[0].Domain != "casa" {
		t.Errorf("Domains = %+v", got)
	}
}

func TestSyncDNSNaoRepeteODominio(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}
	m.dnsBg.Wait()

	fake.Calls = nil
	m.SyncDNS(context.Background())
	if len(fake.Calls) != 1 {
		t.Errorf("updates = %v, queria um só", fake.Calls)
	}
}

func TestSyncDNSAtualizaNomeCadastrado(t *testing.T) {
	m, fake := newDNSManager(t)
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AddDNSDomain(context.Background(), "casa"); err != nil {
		t.Fatal(err)
	}
	m.dnsBg.Wait()

	fake.IP = "200.1.2.3"
	m.SyncDNS(context.Background())

	if got := m.DNS().Domains[0].LastIP; got != "200.1.2.3" {
		t.Errorf("LastIP = %q, queria o IP novo", got)
	}
}
