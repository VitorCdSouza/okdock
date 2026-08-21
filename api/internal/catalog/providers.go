package catalog

import "sort"

const CustomProviderID = "custom"

func ptr(f float64) *float64 { return &f }

func opts(pairs ...string) []Option {
	out := make([]Option, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, Option{Value: pairs[i], Label: pairs[i+1]})
	}
	return out
}

var providers = []Provider{
	{
		ID:               "itzg/minecraft-server",
		Game:             "minecraft-java",
		GameLabel:        "Minecraft (Java)",
		Short:            "MC",
		Image:            "itzg/minecraft-server:java21",
		Description:      "Servidor Java com suporte a vanilla, Paper, Fabric, Forge e modpacks.",
		Docs:             "https://docker-minecraft-server.readthedocs.io/",
		Ports:            []Port{{Container: 25565, Protocol: "tcp", DefaultHost: 25565, Label: "Jogo"}},
		Volumes:          []Volume{{Host: "./data", Container: "/data", Data: true}},
		DefaultMemory:    "4g",
		MinMemory:        "2g",
		DefaultCPUs:      2,
		StopGraceSeconds: 120,
		Fields: []Field{
			{Key: "EULA", Label: "EULA da Mojang", Type: FieldBool, Default: "true", Required: true,
				Help: "A imagem não sobe sem isto aceito."},
			{Key: "TYPE", Label: "Distribuição", Type: FieldEnum, Default: "VANILLA",
				Options: opts("VANILLA", "Vanilla", "PAPER", "Paper", "FABRIC", "Fabric", "FORGE", "Forge", "SPIGOT", "Spigot")},
			{Key: "VERSION", Label: "Versão", Type: FieldText, Default: "LATEST",
				Help: "LATEST, SNAPSHOT ou uma versão exata como 1.21.1."},
			{Key: "MEMORY", Label: "Heap da JVM", Type: FieldText, Default: "3G",
				Help: "Deixe pelo menos 1 GB abaixo do limite de RAM do container, senão o kernel mata o processo (exit 137)."},
			{Key: "MOTD", Label: "MOTD", Type: FieldText, Default: "Servidor GameDock"},
			{Key: "DIFFICULTY", Label: "Dificuldade", Type: FieldEnum, Default: "normal",
				Options: opts("peaceful", "Pacífico", "easy", "Fácil", "normal", "Normal", "hard", "Difícil")},
			{Key: "MODE", Label: "Modo de jogo", Type: FieldEnum, Default: "survival",
				Options: opts("survival", "Sobrevivência", "creative", "Criativo", "adventure", "Aventura", "spectator", "Espectador")},
			{Key: "MAX_PLAYERS", Label: "Máximo de jogadores", Type: FieldInt, Default: "10", Min: ptr(1), Max: ptr(200)},
			{Key: "ONLINE_MODE", Label: "Exigir conta Mojang", Type: FieldBool, Default: "true"},
			{Key: "PVP", Label: "PvP", Type: FieldBool, Default: "true"},
			{Key: "OPS", Label: "Operadores", Type: FieldText, Help: "Nicks separados por vírgula."},
			{Key: "LEVEL_SEED", Label: "Seed do mundo", Type: FieldText, Advanced: true},
			{Key: "VIEW_DISTANCE", Label: "Distância de render", Type: FieldInt, Default: "10", Min: ptr(3), Max: ptr(32), Advanced: true},
			{Key: "ENABLE_ROLLING_LOGS", Label: "Rotacionar logs", Type: FieldBool, Default: "true", Advanced: true},
		},
	},
	{
		ID:               "ryshe/terraria",
		Game:             "terraria",
		GameLabel:        "Terraria",
		Short:            "TR",
		Image:            "ryshe/terraria:latest",
		Description:      "Servidor de Terraria com TShock.",
		Docs:             "https://github.com/ryshe/docker-terraria",
		Ports:            []Port{{Container: 7777, Protocol: "tcp", DefaultHost: 7777, Label: "Jogo"}},
		Volumes:          []Volume{{Host: "./data", Container: "/root/.local/share/Terraria/Worlds", Data: true}},
		DefaultMemory:    "2g",
		MinMemory:        "512m",
		DefaultCPUs:      2,
		StopGraceSeconds: 60,
		Fields: []Field{
			{Key: "WORLD_FILENAME", Label: "Arquivo do mundo", Type: FieldText, Default: "gamedock.wld", Required: true,
				Help: "Nome do .wld dentro de ./data. Se não existir, é criado com o tamanho abaixo."},
			{Key: "AUTOCREATE", Label: "Tamanho do mundo novo", Type: FieldEnum, Default: "2",
				Target: TargetArg, Flag: "-autocreate",
				Options: opts("1", "Pequeno", "2", "Médio", "3", "Grande"),
				Help:    "Só tem efeito quando o arquivo do mundo ainda não existe."},
			{Key: "WORLDNAME", Label: "Nome do mundo", Type: FieldText, Default: "GameDock",
				Target: TargetArg, Flag: "-worldname"},
			{Key: "DIFFICULTY", Label: "Dificuldade", Type: FieldEnum, Default: "0",
				Target: TargetArg, Flag: "-difficulty",
				Options: opts("0", "Clássico", "1", "Especialista", "2", "Mestre", "3", "Jornada")},
			{Key: "MAXPLAYERS", Label: "Máximo de jogadores", Type: FieldInt, Default: "8", Min: ptr(1), Max: ptr(255),
				Target: TargetArg, Flag: "-maxplayers"},
			{Key: "PASSWORD", Label: "Senha", Type: FieldPassword, Secret: true,
				Target: TargetArg, Flag: "-password"},
			{Key: "MOTD", Label: "MOTD", Type: FieldText,
				Target: TargetArg, Flag: "-motd"},
			{Key: "SEED", Label: "Seed do mundo", Type: FieldText, Advanced: true,
				Target: TargetArg, Flag: "-seed"},
			{Key: "SECURE", Label: "Anticheat", Type: FieldBool, Default: "false", Advanced: true,
				Target: TargetArg, Flag: "-secure"},
			{Key: "NOUPNP", Label: "Desligar UPnP", Type: FieldBool, Default: "true", Advanced: true,
				Target: TargetArg, Flag: "-noupnp",
				Help: "O painel já publica a porta; deixar o servidor mexer no roteador só atrapalha."},
		},
	},
	{
		ID:               CustomProviderID,
		Game:             "custom",
		GameLabel:        "Imagem custom",
		Short:            "··",
		Image:            "",
		Description:      "Qualquer imagem. Você informa portas, volumes e variáveis à mão.",
		Ports:            nil,
		Volumes:          []Volume{{Host: "./data", Container: "/data", Data: true}},
		DefaultMemory:    "2g",
		MinMemory:        "256m",
		DefaultCPUs:      2,
		StopGraceSeconds: 30,
		Fields:           nil,
	},
}

var byID = func() map[string]Provider {
	m := make(map[string]Provider, len(providers))
	for _, p := range providers {
		m[p.ID] = p
	}
	return m
}()

func All() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].ID == CustomProviderID) != (out[j].ID == CustomProviderID) {
			return out[j].ID == CustomProviderID
		}
		return out[i].GameLabel < out[j].GameLabel
	})
	return out
}

func Get(id string) (Provider, bool) {
	p, ok := byID[id]
	return p, ok
}
