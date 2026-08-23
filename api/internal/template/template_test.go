package template

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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
			t.Errorf("template %q is not fully identified", p.ID)
		}
		if p.ID != CustomID && p.Image == "" {
			t.Errorf("template %q sem imagem", p.ID)
		}
		if _, ok := p.DataVolume(); !ok {
			t.Errorf("template %q marks no volume as the world one", p.ID)
		}
		seen := map[string]bool{}
		for _, f := range p.Fields {
			if seen[f.Key] {
				t.Errorf("template %q repete o campo %q", p.ID, f.Key)
			}
			seen[f.Key] = true
			if f.Type == FieldEnum && len(f.Options) == 0 {
				t.Errorf("%s.%s is an enum with no options", p.ID, f.Key)
			}
			if f.Default != "" {
				if _, err := validateField(f, f.Default); err != nil {
					t.Errorf("%s.%s: default %q fails its own validation: %v", p.ID, f.Key, f.Default, err)
				}
			}
		}
	}
}

func TestCustomIsLast(t *testing.T) {
	all := testCatalog(t).All()
	if all[len(all)-1].ID != CustomID {
		t.Fatalf("the custom image should be last in the catalog, got %q", all[len(all)-1].ID)
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
		t.Errorf("default not applied: TYPE=%q", out["TYPE"])
	}
}

func TestValidateRejectsUnknownField(t *testing.T) {
	p, _ := testCatalog(t).Get("minecraft-java")
	_, err := p.Validate(map[string]string{"RM_RF": "sim"})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Error(), "RM_RF") {
		t.Errorf("the error does not mention the field: %v", ve)
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
		t.Error("an enum value outside the options should fail")
	}
	if _, err := p.Validate(map[string]string{"MAX_PLAYERS": "0"}); err == nil {
		t.Error("a value below the minimum should fail")
	}
	if _, err := p.Validate(map[string]string{"MAX_PLAYERS": "muitos"}); err == nil {
		t.Error("a non-numeric integer should fail")
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
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if len(ve.Problems) != 2 {
		t.Errorf("esperava 2 problemas, veio %d: %v", len(ve.Problems), ve.Problems)
	}
}

func TestValidateRequiredMissing(t *testing.T) {
	p, _ := testCatalog(t).Get("minecraft-java")
	_, err := p.Validate(map[string]string{"EULA": ""})
	if err == nil {
		t.Fatal("a blank required field should fail")
	}
	if !strings.Contains(err.Error(), "EULA") {
		t.Errorf("the error does not point at the required field: %v", err)
	}
}

func TestArgFieldsDeclareAFlag(t *testing.T) {
	for _, p := range testCatalog(t).All() {
		for _, f := range p.Fields {
			if f.IsArg() && f.Flag == "" {
				t.Errorf("%s.%s is an argument but does not say which flag", p.ID, f.Key)
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
		t.Errorf("MAXPLAYERS is an argument, it must not be in the environment: %v", env)
	}
	if !containsPair(args, "-maxplayers", "12") {
		t.Errorf("args do not carry -maxplayers 12: %v", args)
	}
	if !containsPair(args, "-autocreate", "2") {
		t.Errorf("args do not carry -autocreate: %v", args)
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
		t.Errorf("a boolean flag carries no value: %v", args)
	}

	values, _ = p.Validate(map[string]string{"SECURE": "false"})
	_, args = p.SplitValues(values)
	if contains(args, "-secure") {
		t.Errorf("a boolean flag turned off should not show up: %v", args)
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
		t.Errorf("expected the ${PASSWORD} reference: %v", args)
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
		t.Fatal("the vanilla template vanished from the catalog")
	}
	if _, exists := p.Field("WORLD_FILENAME"); exists {
		t.Error("WORLD_FILENAME must not exist in this template: filling it keeps the server from starting")
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
		t.Errorf("this image is not configured through the environment, got: %v", env)
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
				t.Errorf("%s uses the moving tag %q: pin a version", p.ID, moving)
			}
		}
		if !strings.Contains(p.Image, ":") {
			t.Errorf("%s has no tag on image %q", p.ID, p.Image)
		}
	}
}

func TestAcceptsImageSeparatesTerrariaVariants(t *testing.T) {
	tshock, _ := testCatalog(t).Get("terraria-tshock")
	vanilla, _ := testCatalog(t).Get("terraria-vanilla")

	if !tshock.AcceptsImage("ryshe/terraria:tshock-1.4.5.6-6.1.0") {
		t.Error("the TShock template should accept its own image")
	}
	if !vanilla.AcceptsImage("ryshe/terraria:vanilla-1.4.5.7") {
		t.Error("the vanilla template should accept its own image")
	}
	if tshock.AcceptsImage("ryshe/terraria:vanilla-1.4.5.7") {
		t.Error("the TShock template must not accept the vanilla image")
	}
	if vanilla.AcceptsImage("ryshe/terraria:tshock-1.4.5.6-6.1.0") {
		t.Error("the vanilla template must not accept the TShock image")
	}
}

func TestAcceptsImageAllowsNewerTagsOfTheSameVariant(t *testing.T) {
	p, _ := testCatalog(t).Get("terraria-vanilla")
	if !p.AcceptsImage("ryshe/terraria:vanilla-1.4.6.0") {
		t.Error("a newer version of the same variant should be accepted")
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
			t.Errorf("%s: invalid pattern %q: %v", p.ID, p.ImagePattern, err)
			continue
		}
		if !p.AcceptsImage(p.Image) {
			t.Errorf("%s: pattern %q rejects the default image %q", p.ID, p.ImagePattern, p.Image)
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
		t.Errorf("an image outside the catalog should not match %q", p.ID)
	}
}

func TestTerrariaLabelsNameTheVariant(t *testing.T) {
	for _, id := range []string{"terraria-tshock", "terraria-vanilla"} {
		p, _ := testCatalog(t).Get(id)
		if !strings.Contains(p.Name, "(") {
			t.Errorf("%s: label %q does not say which variant it is", id, p.Name)
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
		t.Fatalf("expected ValidationError, got %v", err)
	}

	byField := map[string]Problem{}
	for _, pr := range ve.Problems {
		byField[pr.Field] = pr
	}

	if got := byField["DIFFICULTY"].Code; got != "not_option" {
		t.Errorf("code de DIFFICULTY = %q, queria not_option", got)
	}
	if got := byField["DIFFICULTY"].Params["value"]; got != "impossivel" {
		t.Errorf("DIFFICULTY params do not carry the rejected value: %v", byField["DIFFICULTY"].Params)
	}
	if got := byField["MAX_PLAYERS"].Code; got != "below_min" {
		t.Errorf("code de MAX_PLAYERS = %q, queria below_min", got)
	}
	if got := byField["MAX_PLAYERS"].Params["min"]; got != 1.0 {
		t.Errorf("MAX_PLAYERS params do not carry the minimum: %v", byField["MAX_PLAYERS"].Params)
	}
}

func TestGetAcceptsTheOldIDFromTheSpec(t *testing.T) {
	c := testCatalog(t)
	for old, want := range map[string]string{
		"itzg/minecraft-server":  "minecraft-java",
		"ryshe/terraria":         "terraria-tshock",
		"ryshe/terraria-vanilla": "terraria-vanilla",
	} {
		got, ok := c.Get(old)
		if !ok || got.ID != want {
			t.Errorf("Get(%q) = %q, %v; wanted %q", old, got.ID, ok, want)
		}
	}
}

func TestEveryBuiltinTemplateHasAKnownCategory(t *testing.T) {
	for _, tmpl := range testCatalog(t).All() {
		if !tmpl.Category.Valid() {
			t.Errorf("%s: category %q is not in the list", tmpl.ID, tmpl.Category)
		}
		if !tmpl.Builtin {
			t.Errorf("%s should be marked as builtin", tmpl.ID)
		}
	}
}

func TestSaveWritesToDiskAndEditsABuiltinTemplate(t *testing.T) {
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
		t.Errorf("did not write the file: %v", err)
	}
	got, _ := c.Get("minecraft-java")
	if got.DefaultMemory != "8g" {
		t.Errorf("the edit did not stick: %q", got.DefaultMemory)
	}
	if got.Builtin {
		t.Error("an edited template is no longer builtin")
	}

	other, err := NewCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := other.Get("minecraft-java"); again.DefaultMemory != "8g" {
		t.Errorf("the edit did not survive a restart: %q", again.DefaultMemory)
	}

	if err := c.Delete("minecraft-java"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	back, ok := c.Get("minecraft-java")
	if !ok || back.DefaultMemory != original.DefaultMemory || !back.Builtin {
		t.Errorf("deleting the edit should bring the builtin back: %+v", back)
	}
}

func TestDeleteRefusesAnUneditedBuiltinTemplate(t *testing.T) {
	c := testCatalog(t)
	if err := c.Delete("minecraft-java"); !errors.Is(err, ErrBuiltin) {
		t.Errorf("wanted ErrBuiltin, got %v", err)
	}
	if err := c.Delete("does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Errorf("wanted ErrNotFound, got %v", err)
	}
}

func TestSaveRefusesAnInvalidTemplate(t *testing.T) {
	c := testCatalog(t)
	err := c.Save(Template{ID: "../escape", Name: "", Category: "Filmes!"})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	byField := map[string]string{}
	for _, p := range ve.Problems {
		byField[p.Field] = p.Code
	}
	for field, code := range map[string]string{"id": "bad_template_id", "name": "required", "category": "unknown_category"} {
		if byField[field] != code {
			t.Errorf("problem for %s = %q, wanted %q (%v)", field, byField[field], code, ve.Problems)
		}
	}
	if _, err := os.Stat(filepath.Join(c.dir, "..", "escape.json")); err == nil {
		t.Error("a template with an invalid id must not become a file")
	}
}

func TestSaveAcceptsACategoryOfItsOwn(t *testing.T) {
	c := testCatalog(t)
	err := c.Save(Template{
		ID: "jellyfin", Name: "Jellyfin", Category: "streaming", Image: "jellyfin/jellyfin:10.9",
		DefaultMemory: "2g", MinMemory: "512m", DefaultCPUs: 2,
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	cats := c.Categories()
	if len(cats) != len(AllCategories)+1 {
		t.Fatalf("categories = %v, wanted the shipped ones plus streaming", cats)
	}
	if cats[len(cats)-2] != "streaming" {
		t.Errorf("streaming should come right before other, got %v", cats)
	}
	if cats[len(cats)-1] != CategoryOther {
		t.Errorf("other has to close the list, got %v", cats)
	}
}

func TestSaveRefusesABrokenField(t *testing.T) {
	c := testCatalog(t)
	err := c.Save(Template{
		ID: "jellyfin", Name: "Jellyfin", Category: CategoryMedia, Image: "jellyfin/jellyfin:10.9",
		Fields: []Field{
			{Key: "MODE", Type: FieldEnum},
			{Key: "MODE", Type: FieldText},
			{Key: "PORT", Type: FieldInt, Default: "many"},
		},
	})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	codes := map[string]bool{}
	for _, p := range ve.Problems {
		codes[p.Code] = true
	}
	for _, want := range []string{"enum_without_options", "duplicate_field", "not_int"} {
		if !codes[want] {
			t.Errorf("missing problem %q: %v", want, ve.Problems)
		}
	}
}

func TestReloadSkipsAnUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := NewCatalog(dir)
	if err == nil {
		t.Error("the unreadable file should be reported")
	}
	if c == nil {
		t.Fatal("a broken template must not take the catalog down")
	}
	if _, ok := c.Get("minecraft-java"); !ok {
		t.Error("the builtin templates must keep working")
	}
}

func TestGuessCategoryFromTheImageName(t *testing.T) {
	cases := map[string]Category{
		"jellyfin/jellyfin:latest":     CategoryMedia,
		"lscr.io/linuxserver/sonarr":   CategoryMedia,
		"mariadb:10.6":                 CategoryDatabase,
		"redis:alpine":                 CategoryDatabase,
		"linuxserver/duckdns":          CategoryNetwork,
		"nextcloud:apache":             CategoryUtilities,
		"itzg/minecraft-server:java21": CategoryGames,
		"ghcr.io/owner/thing:1.0":      CategoryOther,
	}
	for image, want := range cases {
		got, known := GuessCategory(Hints{Image: image})
		if got != want {
			t.Errorf("GuessCategory(%q) = %q, wanted %q", image, got, want)
		}
		if known != (want != CategoryOther) {
			t.Errorf("GuessCategory(%q): known = %v", image, known)
		}
	}
}

func TestGuessCategoryFlaresolverrIsNotMedia(t *testing.T) {
	got, _ := GuessCategory(Hints{Image: "ghcr.io/flaresolverr/flaresolverr:latest"})
	if got != CategoryNetwork {
		t.Errorf("flaresolverr = %q, wanted %q: it is a proxy, not a library", got, CategoryNetwork)
	}
}

func TestGuessCategoryWhenTheImageNameSaysNothing(t *testing.T) {
	cases := []struct {
		name  string
		hints Hints
		want  Category
	}{
		{
			"image label",
			Hints{
				Image: "registry.local/internal:1",
				Labels: map[string]string{
					"org.opencontainers.image.source": "https://github.com/jellyfin/jellyfin",
				},
			},
			CategoryMedia,
		},
		{
			"published port",
			Hints{Image: "registry.local/server:1", Ports: []int{25565}},
			CategoryGames,
		},
		{
			"container name",
			Hints{Image: "registry.local/db:1", Name: "postgres-for-nextcloud"},
			CategoryDatabase,
		},
		{
			"homemade app word",
			Hints{Image: "vitorcds/telegram-promo-bot:1.0"},
			CategoryUtilities,
		},
	}
	for _, c := range cases {
		if got, _ := GuessCategory(c.hints); got != c.want {
			t.Errorf("%s: category = %q, wanted %q", c.name, got, c.want)
		}
	}
}

func TestGuessCategoryIgnoresLabelsThatDoNotDescribeTheImage(t *testing.T) {
	got, known := GuessCategory(Hints{
		Image:  "registry.local/internal:1",
		Labels: map[string]string{"traefik.enable": "true", "traefik.http.routers.x.rule": "Host(`x`)"},
	})
	if known {
		t.Errorf("sitting behind traefik does not make a container network: %q", got)
	}
}

func TestGuessCategoryDoesNotMatchAWordInsideAnother(t *testing.T) {
	if _, known := GuessCategory(Hints{Image: "registry.local/robot-framework:1"}); known {
		t.Error(`"robot" must not match the word "bot"`)
	}
}

func TestCategoryForPrefersTheTemplate(t *testing.T) {
	c := testCatalog(t)

	if got := c.CategoryFor(Hints{Image: "itzg/minecraft-server:java21"}); got != CategoryGames {
		t.Errorf("category = %q", got)
	}
	if got := c.CategoryFor(Hints{Image: "jellyfin/jellyfin:10.9"}); got != CategoryMedia {
		t.Errorf("category = %q", got)
	}
	if got := c.CategoryFor(Hints{Image: "thing/unknown:1"}); got != CategoryOther {
		t.Errorf("category = %q", got)
	}
}
