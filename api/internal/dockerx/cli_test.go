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

func TestParseHostPS(t *testing.T) {
	// Uma linha por container, como o docker ps --format json entrega.
	out := []byte(`{"Names":"jellyfin","Image":"jellyfin/jellyfin:latest","State":"running","Status":"Up 35 hours (healthy)","Ports":"0.0.0.0:8096->8096/tcp, [::]:8096->8096/tcp","Labels":"com.docker.compose.project=media,com.docker.compose.service=jellyfin,com.docker.compose.project.working_dir=/home/vitorcds/servidor/media"}
{"Names":"nextcloud-mysql","Image":"mariadb:10.6","State":"exited","Status":"Exited (137) 2 hours ago","Ports":"3306/tcp","Labels":"com.docker.compose.project=nextcloud,com.docker.compose.service=db"}`)

	list, err := parseHostPS(out)
	if err != nil {
		t.Fatalf("parseHostPS: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("esperava 2 containers, veio %d", len(list))
	}

	jelly := list[0]
	if jelly.Name != "jellyfin" || jelly.Project != "media" || jelly.Service != "jellyfin" {
		t.Errorf("identificação errada: %+v", jelly)
	}
	if jelly.WorkDir != "/home/vitorcds/servidor/media" {
		t.Errorf("workDir = %q", jelly.WorkDir)
	}
	if jelly.Health != "healthy" {
		t.Errorf("health = %q", jelly.Health)
	}
	if len(jelly.Ports) != 1 {
		t.Fatalf("o docker publica a mesma porta em IPv4 e IPv6; para a tela é uma só: %+v", jelly.Ports)
	}
	if jelly.Ports[0] != (HostPort{Host: 8096, Container: 8096, Protocol: "tcp"}) {
		t.Errorf("porta = %+v", jelly.Ports[0])
	}

	db := list[1]
	if db.ExitCode != 137 {
		t.Errorf("exitCode = %d, queria 137 tirado do Status", db.ExitCode)
	}
	if len(db.Ports) != 0 {
		t.Errorf("porta exposta e não publicada não chega ao host: %+v", db.Ports)
	}
}
