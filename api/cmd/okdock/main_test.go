package main

import "testing"

func TestEnvAcceptsTheOldName(t *testing.T) {
	t.Setenv("GAMEDOCK_ROOT", "/srv/velho")

	if got := env("OKDOCK_ROOT", "/containers"); got != "/srv/velho" {
		t.Errorf("env = %q, queria o valor de GAMEDOCK_ROOT", got)
	}

	t.Setenv("OKDOCK_ROOT", "/srv/novo")
	if got := env("OKDOCK_ROOT", "/containers"); got != "/srv/novo" {
		t.Errorf("env = %q, o nome novo devia ganhar do antigo", got)
	}
}

func TestEnvFallsBackToTheDefault(t *testing.T) {
	if got := env("OKDOCK_ROOT", "/containers"); got != "/containers" {
		t.Errorf("env = %q, wanted the default", got)
	}
}
