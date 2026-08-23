package instance

import (
	"encoding/json"
	"testing"
)

func TestValidateName(t *testing.T) {
	ok := []string{"smp", "smp-familia", "mc_1", "a1", "palworld-guilda-2"}
	for _, n := range ok {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, queria nil", n, err)
		}
	}
	bad := []string{"", "a", "SMP", "smp familia", "smp.familia", "../etc", "-smp", "smp/sub"}
	for _, n := range bad {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) devia falhar", n)
		}
	}
}

func TestParseMemory(t *testing.T) {
	cases := map[string]int64{
		"6g":    6 << 30,
		"512m":  512 << 20,
		"1024k": 1024 << 10,
		"2GB":   2 << 30,
		"1024":  1024,
		"":      0,
	}
	for in, want := range cases {
		got, err := ParseMemory(in)
		if err != nil {
			t.Errorf("ParseMemory(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseMemory(%q) = %d, queria %d", in, got, want)
		}
	}
	for _, in := range []string{"muito", "-4g", "0"} {
		if _, err := ParseMemory(in); err == nil {
			t.Errorf("ParseMemory(%q) devia falhar", in)
		}
	}
}

func TestFormatMemoryRoundTrips(t *testing.T) {
	for _, in := range []string{"6g", "512m", "12g"} {
		n, err := ParseMemory(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := FormatMemory(n); got != in {
			t.Errorf("FormatMemory(ParseMemory(%q)) = %q", in, got)
		}
	}
}

func TestPortBindingString(t *testing.T) {
	p := PortBinding{Host: 8211, Container: 8211, Protocol: "udp"}
	if got := p.String(); got != "8211:8211/udp" {
		t.Errorf("String() = %q", got)
	}
}

func TestSpecReadsFieldsWithTheOldName(t *testing.T) {
	raw := []byte(`{"name":"smp","providerId":"itzg/minecraft-server","game":"minecraft-java","image":"itzg/minecraft-server:java21"}`)

	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if spec.TemplateID != "itzg/minecraft-server" {
		t.Errorf("templateId = %q, the old id must survive so the catalog can translate it", spec.TemplateID)
	}
	if spec.Category != "games" {
		t.Errorf("category = %q, an instance created as GameDock could only be a game", spec.Category)
	}
}

func TestSpecPrefersTheNewFields(t *testing.T) {
	raw := []byte(`{"name":"filmes","templateId":"jellyfin","category":"media","providerId":"velho","game":"minecraft-java"}`)

	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if spec.TemplateID != "jellyfin" || spec.Category != "media" {
		t.Errorf("campo novo devia ganhar: %+v", spec)
	}
}
