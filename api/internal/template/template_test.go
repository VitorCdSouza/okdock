package template

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// testCatalog devolve so os templates de fabrica: o diretorio e vazio.
func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := NewCatalog(t.TempDir())
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return c
}

func TestAllBuiltinTemplatesAreUsable(t *testing.T) {
	for _, p := range testCatalog(t).All() {
		if p.ID == "" || p.Name == "" || p.Short == "" {
			t.Errorf("template %q sem identificação completa", p.ID)
		}
		if p.ID != CustomID && p.Image == "" {
			t.Errorf("template %q sem imagem", p.ID)
		}
		if _, ok := p.DataVolume(); !ok {
			t.Errorf("template %q não marca nenhum volume como o do mundo", p.ID)
		}
		seen := map[string]bool{}
		for _, f := range p.Fields {
			if seen[f.Key] {
				t.Errorf("template %q repete o campo %q", p.ID, f.Key)
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
	all := testCatalog(t).All()
	if all[len(all)-1].ID != CustomID {
		t.Fatalf("imagem custom devia ser a última do catálogo, veio %q", all[len(all)-1].ID)
	}
}

func TestValidateAppliesDefaults(t *testing.T) {
	p, _ := testCatalog(t).Get("minecraft-java")
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
	p, _ := testCatalog(t).Get("minecraft-java")
	_, err := p.Validate(map[string]string{"RM_RF": "sim"})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperava ValidationError, veio %v", err)
	}
	if !strings.Contains(ve.Error(), "RM_RF") {
		t.Errorf("erro não menciona o campo: %v", ve)
	}
}

func TestCustomTemplateAcceptsFreeEnv(t *testing.T) {
	p, _ := testCatalog(t).Get(CustomID)
	out, err := p.Validate(map[string]string{"QUALQUER_COISA": "1"})
	if err != nil {
		t.Fatalf("template custom devia aceitar env livre: %v", err)
	}
	if out["QUALQUER_COISA"] != "1" {
		t.Errorf("valor perdido: %v", out)
	}
}

func TestValidateEnumAndRange(t *testing.T) {
	p, _ := testCatalog(t).Get("minecraft-java")

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
	p, _ := testCatalog(t).Get("minecraft-java")
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
	p, _ := testCatalog(t).Get("minecraft-java")
	_, err := p.Validate(map[string]string{"EULA": ""})
	if err == nil {
		t.Fatal("campo obrigatório em branco devia falhar")
	}
	if !strings.Contains(err.Error(), "EULA") {
		t.Errorf("erro não aponta o campo obrigatório: %v", err)
	}
}

func TestArgFieldsDeclareAFlag(t *testing.T) {
	for _, p := range testCatalog(t).All() {
		for _, f := range p.Fields {
			if f.IsArg() && f.Flag == "" {
				t.Errorf("%s.%s é argumento mas não diz qual flag", p.ID, f.Key)
			}
			if !f.IsArg() && f.Flag != "" {
				t.Errorf("%s.%s declara flag mas vai por ambiente", p.ID, f.Key)
			}
		}
	}
}

func TestSplitValuesSeparatesEnvFromArgs(t *testing.T) {
	p, _ := testCatalog(t).Get("terraria-tshock")
	values, err := p.Validate(map[string]string{"MAXPLAYERS": "12"})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	env, args := p.SplitValues(values)

	if env["WORLD_FILENAME"] == "" {
		t.Errorf("WORLD_FILENAME devia ir por ambiente: %v", env)
	}
	if _, ok := env["MAXPLAYERS"]; ok {
		t.Errorf("MAXPLAYERS é argumento, não podia estar no ambiente: %v", env)
	}
	if !containsPair(args, "-maxplayers", "12") {
		t.Errorf("args não têm -maxplayers 12: %v", args)
	}
	if !containsPair(args, "-autocreate", "2") {
		t.Errorf("args não têm -autocreate: %v", args)
	}
}

func TestSplitValuesBoolFlagCarriesNoValue(t *testing.T) {
	p, _ := testCatalog(t).Get("terraria-tshock")

	values, _ := p.Validate(map[string]string{"SECURE": "true"})
	_, args := p.SplitValues(values)
	if !contains(args, "-secure") {
		t.Errorf("flag booleana ligada devia aparecer: %v", args)
	}
	if containsPair(args, "-secure", "true") {
		t.Errorf("flag booleana não leva valor: %v", args)
	}

	values, _ = p.Validate(map[string]string{"SECURE": "false"})
	_, args = p.SplitValues(values)
	if contains(args, "-secure") {
		t.Errorf("flag booleana desligada não devia aparecer: %v", args)
	}
}

func TestSecretArgBecomesInterpolationReference(t *testing.T) {
	p, _ := testCatalog(t).Get("terraria-tshock")
	values, _ := p.Validate(map[string]string{"PASSWORD": "hunter2"})
	env, args := p.SplitValues(values)

	for _, a := range args {
		if a == "hunter2" {
			t.Fatalf("senha vazou para os argumentos: %v", args)
		}
	}
	if !containsPair(args, "-password", "${PASSWORD}") {
		t.Errorf("esperava a referência ${PASSWORD}: %v", args)
	}
	if env["PASSWORD"] != "hunter2" {
		t.Errorf("o valor precisa continuar indo para o .env: %v", env)
	}
}

func TestSplitValuesIsDeterministic(t *testing.T) {
	p, _ := testCatalog(t).Get("terraria-tshock")
	values, _ := p.Validate(map[string]string{"MAXPLAYERS": "12", "MOTD": "oi"})

	_, first := p.SplitValues(values)
	for i := 0; i < 20; i++ {
		_, again := p.SplitValues(values)
		if strings.Join(first, " ") != strings.Join(again, " ") {
			t.Fatalf("ordem dos argumentos mudou:\n%v\n%v", first, again)
		}
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func containsPair(list []string, flag, value string) bool {
	for i := 0; i+1 < len(list); i++ {
		if list[i] == flag && list[i+1] == value {
			return true
		}
	}
	return false
}

func TestTerrariaVanillaKeepsWorldOutOfEnvironment(t *testing.T) {
	p, ok := testCatalog(t).Get("terraria-vanilla")
	if !ok {
		t.Fatal("template vanilla sumiu do catálogo")
	}
	if _, exists := p.Field("WORLD_FILENAME"); exists {
		t.Error("WORLD_FILENAME não pode existir neste template: preenchê-la impede o servidor de subir")
	}
	world, ok := p.Field("WORLD")
	if !ok || !world.IsArg() || world.Flag != "-world" {
		t.Errorf("o mundo precisa ir por -world: %+v", world)
	}

	values, err := p.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	env, args := p.SplitValues(values)
	if len(env) != 0 {
		t.Errorf("esta imagem não se configura por ambiente, veio: %v", env)
	}
	if !containsPair(args, "-autocreate", "2") {
		t.Errorf("sem -autocreate a imagem vanilla sai com erro em mundo novo: %v", args)
	}
}

func TestGameImagesArePinned(t *testing.T) {
	for _, p := range testCatalog(t).All() {
		if p.ID == CustomID {
			continue
		}
		for _, moving := range []string{":latest", ":stable"} {
			if strings.HasSuffix(p.Image, moving) {
				t.Errorf("%s usa a tag móvel %q: fixe uma versão", p.ID, moving)
			}
		}
		if !strings.Contains(p.Image, ":") {
			t.Errorf("%s não tem tag na imagem %q", p.ID, p.Image)
		}
	}
}

func TestAcceptsImageSeparatesTerrariaVariants(t *testing.T) {
	tshock, _ := testCatalog(t).Get("terraria-tshock")
	vanilla, _ := testCatalog(t).Get("terraria-vanilla")

	if !tshock.AcceptsImage("ryshe/terraria:tshock-1.4.5.6-6.1.0") {
		t.Error("template TShock devia aceitar a própria imagem")
	}
	if !vanilla.AcceptsImage("ryshe/terraria:vanilla-1.4.5.7") {
		t.Error("template vanilla devia aceitar a própria imagem")
	}
	if tshock.AcceptsImage("ryshe/terraria:vanilla-1.4.5.7") {
		t.Error("template TShock não pode aceitar a imagem vanilla")
	}
	if vanilla.AcceptsImage("ryshe/terraria:tshock-1.4.5.6-6.1.0") {
		t.Error("template vanilla não pode aceitar a imagem TShock")
	}
}

func TestAcceptsImageAllowsNewerTagsOfTheSameVariant(t *testing.T) {
	p, _ := testCatalog(t).Get("terraria-vanilla")
	if !p.AcceptsImage("ryshe/terraria:vanilla-1.4.6.0") {
		t.Error("versão nova da mesma variante devia ser aceita")
	}
}

func TestCustomTemplateAcceptsAnyImage(t *testing.T) {
	p, _ := testCatalog(t).Get(CustomID)
	if !p.AcceptsImage("qualquer/coisa:1") {
		t.Error("o template custom precisa aceitar qualquer imagem")
	}
}

func TestImagePatternsCompileAndMatchTheirOwnDefault(t *testing.T) {
	for _, p := range testCatalog(t).All() {
		if p.ImagePattern == "" {
			continue
		}
		if _, err := regexp.Compile(p.ImagePattern); err != nil {
			t.Errorf("%s: padrão inválido %q: %v", p.ID, p.ImagePattern, err)
			continue
		}
		if !p.AcceptsImage(p.Image) {
			t.Errorf("%s: o padrão %q rejeita a imagem padrão %q", p.ID, p.ImagePattern, p.Image)
		}
	}
}

func TestTemplateForImageFindsTheRightVariant(t *testing.T) {
	p, ok := testCatalog(t).TemplateForImage("ryshe/terraria:vanilla-1.4.5.7")
	if !ok || p.ID != "terraria-vanilla" {
		t.Fatalf("TemplateForImage = %q, %v", p.ID, ok)
	}
	p, ok = testCatalog(t).TemplateForImage("ryshe/terraria:tshock-1.4.5.6-6.1.0")
	if !ok || p.ID != "terraria-tshock" {
		t.Fatalf("TemplateForImage = %q, %v", p.ID, ok)
	}
	if p, ok := testCatalog(t).TemplateForImage("nginx:alpine"); ok {
		t.Errorf("imagem fora do catálogo não devia casar com %q", p.ID)
	}
}

func TestTerrariaLabelsNameTheVariant(t *testing.T) {
	for _, id := range []string{"terraria-tshock", "terraria-vanilla"} {
		p, _ := testCatalog(t).Get(id)
		if !strings.Contains(p.Name, "(") {
			t.Errorf("%s: o rótulo %q não diz qual variante é", id, p.Name)
		}
	}
}

func TestValidateProblemsCarryCodeAndParams(t *testing.T) {
	p, _ := testCatalog(t).Get("minecraft-java")
	_, err := p.Validate(map[string]string{
		"DIFFICULTY":  "impossivel",
		"MAX_PLAYERS": "0",
	})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperava ValidationError, veio %v", err)
	}

	byField := map[string]Problem{}
	for _, pr := range ve.Problems {
		byField[pr.Field] = pr
	}

	if got := byField["DIFFICULTY"].Code; got != "not_option" {
		t.Errorf("code de DIFFICULTY = %q, queria not_option", got)
	}
	if got := byField["DIFFICULTY"].Params["value"]; got != "impossivel" {
		t.Errorf("params de DIFFICULTY não levam o valor recusado: %v", byField["DIFFICULTY"].Params)
	}
	if got := byField["MAX_PLAYERS"].Code; got != "below_min" {
		t.Errorf("code de MAX_PLAYERS = %q, queria below_min", got)
	}
	if got := byField["MAX_PLAYERS"].Params["min"]; got != 1.0 {
		t.Errorf("params de MAX_PLAYERS não levam o mínimo: %v", byField["MAX_PLAYERS"].Params)
	}
}

func TestGetAceitaOIDAntigoDaSpec(t *testing.T) {
	c := testCatalog(t)
	for old, want := range map[string]string{
		"itzg/minecraft-server":  "minecraft-java",
		"ryshe/terraria":         "terraria-tshock",
		"ryshe/terraria-vanilla": "terraria-vanilla",
	} {
		got, ok := c.Get(old)
		if !ok || got.ID != want {
			t.Errorf("Get(%q) = %q, %v; queria %q", old, got.ID, ok, want)
		}
	}
}

func TestTodoTemplateDeFabricaTemCategoriaConhecida(t *testing.T) {
	for _, tmpl := range testCatalog(t).All() {
		if !tmpl.Category.Valid() {
			t.Errorf("%s: categoria %q não está na lista", tmpl.ID, tmpl.Category)
		}
		if !tmpl.Builtin {
			t.Errorf("%s devia estar marcado como de fábrica", tmpl.ID)
		}
	}
}

func TestSaveGravaEmDiscoEEditaTemplateDeFabrica(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}

	original, _ := c.Get("minecraft-java")
	edited := original
	edited.Builtin = false
	edited.DefaultMemory = "8g"
	if err := c.Save(edited); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "minecraft-java.json")); err != nil {
		t.Errorf("não gravou o arquivo: %v", err)
	}
	got, _ := c.Get("minecraft-java")
	if got.DefaultMemory != "8g" {
		t.Errorf("a edição não valeu: %q", got.DefaultMemory)
	}
	if got.Builtin {
		t.Error("template editado não é mais de fábrica")
	}

	// Relendo do zero, a edicao continua valendo.
	outro, err := NewCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := outro.Get("minecraft-java"); again.DefaultMemory != "8g" {
		t.Errorf("a edição não sobreviveu ao restart: %q", again.DefaultMemory)
	}

	// E apagar devolve o de fabrica em vez de sumir com o template.
	if err := c.Delete("minecraft-java"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	back, ok := c.Get("minecraft-java")
	if !ok || back.DefaultMemory != original.DefaultMemory || !back.Builtin {
		t.Errorf("apagar a edição devia devolver o de fábrica: %+v", back)
	}
}

func TestDeleteRecusaTemplateDeFabricaSemEdicao(t *testing.T) {
	c := testCatalog(t)
	if err := c.Delete("minecraft-java"); !errors.Is(err, ErrBuiltin) {
		t.Errorf("queria ErrBuiltin, veio %v", err)
	}
	if err := c.Delete("nao-existe"); !errors.Is(err, ErrNotFound) {
		t.Errorf("queria ErrNotFound, veio %v", err)
	}
}

func TestSaveRecusaTemplateInvalido(t *testing.T) {
	c := testCatalog(t)
	err := c.Save(Template{ID: "../fuga", Name: "", Category: "filmes"})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperava ValidationError, veio %v", err)
	}
	byField := map[string]string{}
	for _, p := range ve.Problems {
		byField[p.Field] = p.Code
	}
	for field, code := range map[string]string{"id": "bad_template_id", "name": "required", "category": "unknown_category"} {
		if byField[field] != code {
			t.Errorf("problema de %s = %q, queria %q (%v)", field, byField[field], code, ve.Problems)
		}
	}
	if _, err := os.Stat(filepath.Join(c.dir, "..", "fuga.json")); err == nil {
		t.Error("template com id inválido não podia ter virado arquivo")
	}
}

func TestSaveRecusaCampoQuebrado(t *testing.T) {
	c := testCatalog(t)
	err := c.Save(Template{
		ID: "jellyfin", Name: "Jellyfin", Category: CategoryMedia, Image: "jellyfin/jellyfin:10.9",
		Fields: []Field{
			{Key: "MODO", Type: FieldEnum},
			{Key: "MODO", Type: FieldText},
			{Key: "PORTA", Type: FieldInt, Default: "muitas"},
		},
	})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperava ValidationError, veio %v", err)
	}
	codes := map[string]bool{}
	for _, p := range ve.Problems {
		codes[p.Code] = true
	}
	for _, want := range []string{"enum_without_options", "duplicate_field", "not_int"} {
		if !codes[want] {
			t.Errorf("faltou o problema %q: %v", want, ve.Problems)
		}
	}
}

func TestReloadIgnoraArquivoIlegivel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "quebrado.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := NewCatalog(dir)
	if err == nil {
		t.Error("o arquivo ilegível devia ser reportado")
	}
	if c == nil {
		t.Fatal("um template quebrado não pode derrubar o catálogo")
	}
	if _, ok := c.Get("minecraft-java"); !ok {
		t.Error("os templates de fábrica precisam continuar valendo")
	}
}
