package duckdns

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNormalize(t *testing.T) {
	ok := map[string]string{
		"smp":                      "smp",
		"  SMP  ":                  "smp",
		"smp.duckdns.org":          "smp",
		"https://smp.duckdns.org/": "smp",
		"casa-do-joao.duckdns.org": "casa-do-joao",
	}
	for in, want := range ok {
		got, err := Normalize(in)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("Normalize(%q) = %q, queria %q", in, got, want)
		}
	}

	for _, in := range []string{"", "-smp", "smp-", "smp.familia", "smp_familia", "smp familia"} {
		if _, err := Normalize(in); !errors.Is(err, ErrInvalidDomain) {
			t.Errorf("Normalize(%q) devia recusar, devolveu %v", in, err)
		}
	}
}

func TestUpdateRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("KO"))
	}))
	defer srv.Close()

	_, err := HTTP{Endpoint: srv.URL}.Update(context.Background(), "token", "smp")
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("queria ErrRejected, veio %v", err)
	}
}

func TestUpdateOK(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte("OK\n187.12.3.4\n\nUPDATED\n"))
	}))
	defer srv.Close()

	res, err := HTTP{Endpoint: srv.URL}.Update(context.Background(), "tok", "smp")
	if err != nil {
		t.Fatal(err)
	}
	if res.IP != "187.12.3.4" || !res.Changed {
		t.Errorf("resposta lida errado: %+v", res)
	}
	if _, ok := got["ip"]; !ok || got.Get("ip") != "" {
		t.Errorf("ip devia ir vazio, foi %q", got.Get("ip"))
	}
	if got.Get("verbose") != "true" {
		t.Error("without verbose=true the answer does not carry the registered IP")
	}
	if got.Get("domains") != "smp" || got.Get("token") != "tok" {
		t.Errorf("query errada: %v", got)
	}
}

func TestUpdateNoChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK\n187.12.3.4\n\nNOCHANGE\n"))
	}))
	defer srv.Close()

	res, err := HTTP{Endpoint: srv.URL}.Update(context.Background(), "tok", "smp")
	if err != nil || res.Changed {
		t.Fatalf("NOCHANGE must not come back as Changed: %+v, %v", res, err)
	}
}

func TestHostname(t *testing.T) {
	if got := Hostname("smp"); got != "smp.duckdns.org" {
		t.Errorf("Hostname(smp) = %q", got)
	}
	if got := Hostname(""); got != "" {
		t.Errorf("with no domain there is no hostname, got %q", got)
	}
}
