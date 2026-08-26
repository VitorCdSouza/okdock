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
	st, err := store.New(store.Config{Dir: t.TempDir(), Root: t.TempDir()})
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

func TestLinkDNSChecksAndSaves(t *testing.T) {
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
		t.Errorf("the domain should come out normalized: %+v", link)
	}
	if link.LastIP != fake.IP {
		t.Errorf("LastIP = %q, wanted %q", link.LastIP, fake.IP)
	}

	inst, err := m.Get(context.Background(), "smp")
	if err != nil {
		t.Fatal(err)
	}
	if inst.DNS == nil || inst.DNS.Hostname != "smp.duckdns.org" {
		t.Fatalf("the instance should carry the DNS: %+v", inst.DNS)
	}
}

func TestLinkDNSWithoutAToken(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")

	if _, err := m.LinkDNS(context.Background(), "smp", "smp"); !errors.Is(err, ErrNoToken) {
		t.Fatalf("wanted ErrNoToken, got %v", err)
	}
	if len(fake.Calls) != 0 {
		t.Error("with no token there is no point calling duckdns")
	}
}

func TestLinkDNSRefusedSavesNothing(t *testing.T) {
	m, fake := newDNSManager(t)
	mustCreate(t, m, "smp")
	fake.Domains = map[string]bool{"other": true}

	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "smp"); !errors.Is(err, duckdns.ErrRejected) {
		t.Fatalf("wanted ErrRejected, got %v", err)
	}
	if len(m.DNS().Links) != 0 {
		t.Error("the link should not have been saved")
	}
}

func TestLinkDNSDomainAlreadyUsed(t *testing.T) {
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
		t.Fatalf("wanted ErrDNSTaken, got %v", err)
	}
}

func TestLinkDNSUnknownInstance(t *testing.T) {
	m, _ := newDNSManager(t)
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "fantasma", "casa"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wanted ErrNotFound, got %v", err)
	}
}

func TestDNSSurvivesBetweenManagers(t *testing.T) {
	m, _ := newDNSManager(t)
	mustCreate(t, m, "smp")
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LinkDNS(context.Background(), "smp", "casa"); err != nil {
		t.Fatal(err)
	}

	other := New(Options{
		Store:  m.store,
		Docker: dockerx.NewFake(),
		System: system.StaticReader{},
		DNS:    duckdns.NewFake(),
	})
	got := other.DNS()
	if got.Token != "tok" || len(got.Links) != 1 || got.Links[0].Domain != "casa" {
		t.Fatalf("the config did not come back from disk: %+v", got)
	}
}

func TestSyncDNSUpdatesTheIP(t *testing.T) {
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
		t.Errorf("LastIP = %q, wanted the new IP", got)
	}
}

func TestSyncDNSKeepsTheFailure(t *testing.T) {
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
		t.Error("the sync failure should be recorded on the link")
	}
}

func TestSyncDNSWithoutATokenNeverHitsTheNetwork(t *testing.T) {
	m, fake := newDNSManager(t)
	m.SyncDNS(context.Background())
	if len(fake.Calls) != 0 {
		t.Error("with no token saved the panel must not call duckdns")
	}
}

func TestDeleteForgetsTheLink(t *testing.T) {
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
		t.Error("an orphan link survived deleting the instance")
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
		t.Fatal("the link should be gone")
	}
	if m.DNS().Token != "tok" {
		t.Error("the token should not vanish with it")
	}
}

func TestDNSIsOffWithoutAClient(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	if err := m.SetDNSToken(context.Background(), "tok"); !errors.Is(err, ErrDNSDisabled) {
		t.Fatalf("wanted ErrDNSDisabled, got %v", err)
	}
}

func TestAddDNSDomainChecksAndSaves(t *testing.T) {
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
		t.Errorf("entry = %+v", entry)
	}
	if got := m.DNS().Domains; len(got) != 1 || got[0].Domain != "casa" {
		t.Errorf("Domains = %+v", got)
	}
}

func TestAddDNSDomainRefusedStaysOutOfTheList(t *testing.T) {
	m, fake := newDNSManager(t)
	fake.Domains = map[string]bool{"casa": true}
	if err := m.SetDNSToken(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}

	if _, err := m.AddDNSDomain(context.Background(), "someone-else"); !errors.Is(err, duckdns.ErrRejected) {
		t.Fatalf("wanted ErrRejected, got %v", err)
	}
	if got := m.DNS().Domains; len(got) != 0 {
		t.Errorf("Domains = %+v, wanted empty", got)
	}
}

func TestAddDNSDomainWithoutAToken(t *testing.T) {
	m, fake := newDNSManager(t)
	if _, err := m.AddDNSDomain(context.Background(), "casa"); !errors.Is(err, ErrNoToken) {
		t.Fatalf("wanted ErrNoToken, got %v", err)
	}
	if len(fake.Calls) != 0 {
		t.Error("with no token the panel must not call duckdns")
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
		t.Errorf("Domains = %+v, wanted empty", got)
	}
}

func TestLinkDNSRegistersTheName(t *testing.T) {
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

func TestSyncDNSDoesNotRepeatTheDomain(t *testing.T) {
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
		t.Errorf("updates = %v, wanted just one", fake.Calls)
	}
}

func TestSyncDNSUpdatesARegisteredName(t *testing.T) {
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
		t.Errorf("LastIP = %q, wanted the new IP", got)
	}
}
