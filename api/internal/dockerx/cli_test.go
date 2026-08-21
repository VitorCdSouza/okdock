package dockerx

import "testing"

func TestParsePSAcceptsBothComposeFormats(t *testing.T) {
	lines := []byte(`{"Name":"smp","Service":"smp","State":"running","Status":"Up 2 hours","Health":"healthy","ExitCode":0}
{"Name":"pal","Service":"pal","State":"exited","Status":"Exited (137) 3 minutes ago","Health":"","ExitCode":137}`)
	got, err := parsePS(lines)
	if err != nil {
		t.Fatalf("parsePS: %v", err)
	}
	if len(got) != 2 || got[0].Name != "smp" || got[1].ExitCode != 137 {
		t.Fatalf("parsePS = %+v", got)
	}

	array := []byte(`[{"Name":"smp","State":"Running","Status":"Up 2 hours"}]`)
	got, err = parsePS(array)
	if err != nil {
		t.Fatalf("parsePS array: %v", err)
	}
	if len(got) != 1 || got[0].State != "running" {
		t.Fatalf("parsePS array = %+v; State precisa vir normalizado em minúsculas", got)
	}
}

func TestParsePSEmpty(t *testing.T) {
	got, err := parsePS([]byte("  \n"))
	if err != nil || got != nil {
		t.Fatalf("parsePS vazio = %v, %v", got, err)
	}
}

func TestParseMemUsage(t *testing.T) {
	used, limit := parseMemUsage("1.938GiB / 12GiB")
	if used == 0 || limit != 12<<30 {
		t.Errorf("parseMemUsage = %d, %d", used, limit)
	}
}

func TestParsePercent(t *testing.T) {
	if got := parsePercent("58.12%"); got != 58.12 {
		t.Errorf("parsePercent = %v", got)
	}
	if got := parsePercent("--"); got != 0 {
		t.Errorf("valor ilegível devia virar 0, veio %v", got)
	}
}

func TestErrorCarriesStderr(t *testing.T) {
	e := &Error{Args: []string{"compose", "up"}, Stderr: "port is already allocated"}
	if got := e.Error(); got == "" || !contains(got, "already allocated") {
		t.Errorf("Error() = %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
