package catalog

import (
	"errors"
	"strings"
	"testing"
)

func TestAllProvidersAreUsable(t *testing.T) {
	for _, p := range All() {
		if p.ID == "" || p.GameLabel == "" || p.Short == "" {
			t.Errorf("provedor %q sem identificação completa", p.ID)
		}
		if p.ID != CustomProviderID && p.Image == "" {
			t.Errorf("provedor %q sem imagem", p.ID)
		}
		if _, ok := p.DataVolume(); !ok {
			t.Errorf("provedor %q não marca nenhum volume como o do mundo", p.ID)
		}
		seen := map[string]bool{}
		for _, f := range p.Fields {
			if seen[f.Key] {
				t.Errorf("provedor %q repete o campo %q", p.ID, f.Key)
			}
			seen[f.Key] = true
			if f.Type == FieldEnum && len(f.Options) == 0 {
				t.Errorf("%s.%s é enum sem opções", p.ID, f.Key)
			}
			if f.Default != "" {
				if _, err := validateField(f, f.Default); err != nil {
					t.Errorf("%s.%s: default %q não passa na própria validação: %v", p.ID, f.Key, f.Default, err)
				}
			}
		}
	}
}

func TestCustomIsLast(t *testing.T) {
	all := All()
	if all[len(all)-1].ID != CustomProviderID {
		t.Fatalf("imagem custom devia ser a última do catálogo, veio %q", all[len(all)-1].ID)
	}
}

func TestValidateAppliesDefaults(t *testing.T) {
	p, _ := Get("itzg/minecraft-server")
	out, err := p.Validate(map[string]string{"MAX_PLAYERS": "20"})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if out["MAX_PLAYERS"] != "20" {
		t.Errorf("valor informado perdido: %q", out["MAX_PLAYERS"])
	}
	if out["TYPE"] != "VANILLA" {
		t.Errorf("default não aplicado: TYPE=%q", out["TYPE"])
	}
}

func TestValidateRejectsUnknownField(t *testing.T) {
	p, _ := Get("itzg/minecraft-server")
	_, err := p.Validate(map[string]string{"RM_RF": "sim"})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperava ValidationError, veio %v", err)
	}
	if !strings.Contains(ve.Error(), "RM_RF") {
		t.Errorf("erro não menciona o campo: %v", ve)
	}
}

func TestCustomProviderAcceptsFreeEnv(t *testing.T) {
	p, _ := Get(CustomProviderID)
	out, err := p.Validate(map[string]string{"QUALQUER_COISA": "1"})
	if err != nil {
		t.Fatalf("provedor custom devia aceitar env livre: %v", err)
	}
	if out["QUALQUER_COISA"] != "1" {
		t.Errorf("valor perdido: %v", out)
	}
}

func TestValidateEnumAndRange(t *testing.T) {
	p, _ := Get("itzg/minecraft-server")

	if _, err := p.Validate(map[string]string{"DIFFICULTY": "impossivel"}); err == nil {
		t.Error("enum fora das opções devia falhar")
	}
	if _, err := p.Validate(map[string]string{"MAX_PLAYERS": "0"}); err == nil {
		t.Error("valor abaixo do mínimo devia falhar")
	}
	if _, err := p.Validate(map[string]string{"MAX_PLAYERS": "muitos"}); err == nil {
		t.Error("inteiro não numérico devia falhar")
	}
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	p, _ := Get("itzg/minecraft-server")
	_, err := p.Validate(map[string]string{
		"DIFFICULTY":  "impossivel",
		"MAX_PLAYERS": "0",
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperava ValidationError, veio %v", err)
	}
	if len(ve.Problems) != 2 {
		t.Errorf("esperava 2 problemas, veio %d: %v", len(ve.Problems), ve.Problems)
	}
}

func TestValidateRequiredMissing(t *testing.T) {
	p, _ := Get("lloesche/valheim-server")
	_, err := p.Validate(map[string]string{})
	if err == nil {
		t.Fatal("campo obrigatório sem valor devia falhar")
	}
	if !strings.Contains(err.Error(), "SERVER_PASS") {
		t.Errorf("erro não aponta o campo obrigatório: %v", err)
	}
}
