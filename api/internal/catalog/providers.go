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
		ID:               "itzg/minecraft-bedrock-server",
		Game:             "minecraft-bedrock",
		GameLabel:        "Minecraft (Bedrock)",
		Short:            "MB",
		Image:            "itzg/minecraft-bedrock-server:latest",
		Description:      "Servidor Bedrock oficial empacotado — para console, mobile e Windows 10.",
		Docs:             "https://github.com/itzg/docker-minecraft-bedrock-server",
		Ports:            []Port{{Container: 19132, Protocol: "udp", DefaultHost: 19132, Label: "Jogo"}},
		Volumes:          []Volume{{Host: "./data", Container: "/data", Data: true}},
		DefaultMemory:    "2g",
		MinMemory:        "1g",
		DefaultCPUs:      2,
		StopGraceSeconds: 60,
		Fields: []Field{
			{Key: "EULA", Label: "EULA da Mojang", Type: FieldBool, Default: "true", Required: true},
			{Key: "VERSION", Label: "Versão", Type: FieldText, Default: "LATEST"},
			{Key: "SERVER_NAME", Label: "Nome do servidor", Type: FieldText, Default: "Servidor GameDock"},
			{Key: "GAMEMODE", Label: "Modo de jogo", Type: FieldEnum, Default: "survival",
				Options: opts("survival", "Sobrevivência", "creative", "Criativo", "adventure", "Aventura")},
			{Key: "DIFFICULTY", Label: "Dificuldade", Type: FieldEnum, Default: "normal",
				Options: opts("peaceful", "Pacífico", "easy", "Fácil", "normal", "Normal", "hard", "Difícil")},
			{Key: "MAX_PLAYERS", Label: "Máximo de jogadores", Type: FieldInt, Default: "10", Min: ptr(1), Max: ptr(30)},
			{Key: "ALLOW_CHEATS", Label: "Permitir cheats", Type: FieldBool, Default: "false"},
			{Key: "ONLINE_MODE", Label: "Exigir conta Xbox Live", Type: FieldBool, Default: "true"},
			{Key: "LEVEL_NAME", Label: "Nome do mundo", Type: FieldText, Default: "Bedrock level"},
			{Key: "LEVEL_SEED", Label: "Seed do mundo", Type: FieldText, Advanced: true},
			{Key: "VIEW_DISTANCE", Label: "Distância de render", Type: FieldInt, Default: "10", Min: ptr(5), Max: ptr(32), Advanced: true},
		},
	},
	{
		ID:          "thijsvanloef/palworld-server-docker",
		Game:        "palworld",
		GameLabel:   "Palworld",
		Short:       "PW",
		Image:       "thijsvanloef/palworld-server-docker:latest",
		Description: "Servidor dedicado de Palworld, com RCON e backup automático.",
		Docs:        "https://palworld-server-docker.loef.dev/",
		Ports: []Port{
			{Container: 8211, Protocol: "udp", DefaultHost: 8211, Label: "Jogo"},
			{Container: 27015, Protocol: "udp", DefaultHost: 27015, Label: "Query", Optional: true},
			{Container: 25575, Protocol: "tcp", DefaultHost: 25575, Label: "RCON", Optional: true},
		},
		Volumes:          []Volume{{Host: "./data", Container: "/palworld", Data: true}},
		DefaultMemory:    "12g",
		MinMemory:        "8g",
		DefaultCPUs:      4,
		StopGraceSeconds: 60,
		Fields: []Field{
			{Key: "SERVER_NAME", Label: "Nome do servidor", Type: FieldText, Default: "Servidor GameDock"},
			{Key: "SERVER_DESCRIPTION", Label: "Descrição", Type: FieldText},
			{Key: "SERVER_PASSWORD", Label: "Senha de entrada", Type: FieldPassword, Secret: true},
			{Key: "ADMIN_PASSWORD", Label: "Senha de admin", Type: FieldPassword, Secret: true},
			{Key: "PLAYERS", Label: "Máximo de jogadores", Type: FieldInt, Default: "16", Min: ptr(1), Max: ptr(32)},
			{Key: "DIFFICULTY", Label: "Dificuldade", Type: FieldEnum, Default: "None",
				Options: opts("None", "Padrão", "Casual", "Casual", "Normal", "Normal", "Hard", "Difícil")},
			{Key: "DAY_TIME_SPEEDRATE", Label: "Velocidade do dia", Type: FieldFloat, Default: "1.0", Min: ptr(0.1), Max: ptr(5)},
			{Key: "NIGHT_TIME_SPEEDRATE", Label: "Velocidade da noite", Type: FieldFloat, Default: "1.0", Min: ptr(0.1), Max: ptr(5)},
			{Key: "PAL_CAPTURE_RATE", Label: "Taxa de captura", Type: FieldFloat, Default: "1.0", Min: ptr(0.5), Max: ptr(2)},
			{Key: "DEATH_PENALTY", Label: "Penalidade por morte", Type: FieldEnum, Default: "All",
				Options: opts("None", "Nenhuma", "Item", "Itens", "ItemAndEquipment", "Itens e equipamento", "All", "Tudo")},
			{Key: "COMMUNITY", Label: "Listar na comunidade", Type: FieldBool, Default: "false"},
			{Key: "MULTITHREADING", Label: "Multithreading", Type: FieldBool, Default: "true"},
			{Key: "RCON_ENABLED", Label: "RCON", Type: FieldBool, Default: "true", Advanced: true,
				Help: "Precisa estar ligado para o console do painel e para o desligamento com save."},
			{Key: "UPDATE_ON_BOOT", Label: "Atualizar ao subir", Type: FieldBool, Default: "true", Advanced: true},
			{Key: "BACKUP_ENABLED", Label: "Backup automático", Type: FieldBool, Default: "true", Advanced: true},
		},
	},
	{
		ID:          "lloesche/valheim-server",
		Game:        "valheim",
		GameLabel:   "Valheim",
		Short:       "VH",
		Image:       "lloesche/valheim-server:latest",
		Description: "Servidor dedicado de Valheim, com backup e atualização automática.",
		Docs:        "https://github.com/lloesche/valheim-server-docker",
		Ports: []Port{
			{Container: 2456, Protocol: "udp", DefaultHost: 2456, Label: "Jogo"},
			{Container: 2457, Protocol: "udp", DefaultHost: 2457, Label: "Query"},
		},
		Volumes: []Volume{
			{Host: "./config", Container: "/config", Data: true},
			{Host: "./data", Container: "/opt/valheim"},
		},
		DefaultMemory:    "4g",
		MinMemory:        "2g",
		DefaultCPUs:      2,
		StopGraceSeconds: 120,
		Fields: []Field{
			{Key: "SERVER_NAME", Label: "Nome do servidor", Type: FieldText, Default: "Servidor GameDock", Required: true},
			{Key: "WORLD_NAME", Label: "Nome do mundo", Type: FieldText, Default: "Dedicated", Required: true},
			{Key: "SERVER_PASS", Label: "Senha", Type: FieldPassword, Secret: true, Required: true,
				Help: "Mínimo 5 caracteres e não pode conter o nome do servidor — o Valheim recusa subir."},
			{Key: "SERVER_PUBLIC", Label: "Listar publicamente", Type: FieldBool, Default: "true"},
			{Key: "BACKUPS", Label: "Backup automático", Type: FieldBool, Default: "true"},
			{Key: "BACKUPS_INTERVAL", Label: "Intervalo de backup (s)", Type: FieldInt, Default: "43200", Min: ptr(3600), Advanced: true},
			{Key: "UPDATE_INTERVAL", Label: "Intervalo de update (s)", Type: FieldInt, Default: "900", Min: ptr(300), Advanced: true},
		},
	},
	{
		ID:          "hermsi/ark-server",
		Game:        "ark",
		GameLabel:   "ARK: Survival Evolved",
		Short:       "AR",
		Image:       "hermsi/ark-server:latest",
		Description: "Servidor dedicado de ARK. Pesado: conte com 30+ GB de disco por mapa.",
		Docs:        "https://github.com/Hermsi1337/docker-ark-server",
		Ports: []Port{
			{Container: 7777, Protocol: "udp", DefaultHost: 7777, Label: "Jogo"},
			{Container: 7778, Protocol: "udp", DefaultHost: 7778, Label: "Raw"},
			{Container: 27015, Protocol: "udp", DefaultHost: 27015, Label: "Query"},
			{Container: 27020, Protocol: "tcp", DefaultHost: 27020, Label: "RCON", Optional: true},
		},
		Volumes:          []Volume{{Host: "./data", Container: "/app", Data: true}},
		DefaultMemory:    "8g",
		MinMemory:        "6g",
		DefaultCPUs:      4,
		StopGraceSeconds: 180,
		Fields: []Field{
			{Key: "SESSION_NAME", Label: "Nome da sessão", Type: FieldText, Default: "Servidor GameDock", Required: true},
			{Key: "SERVER_MAP", Label: "Mapa", Type: FieldEnum, Default: "TheIsland",
				Options: opts("TheIsland", "The Island", "TheCenter", "The Center", "Ragnarok", "Ragnarok", "ScorchedEarth_P", "Scorched Earth", "Aberration_P", "Aberration", "Extinction", "Extinction", "Valguero_P", "Valguero", "CrystalIsles", "Crystal Isles", "Fjordur", "Fjordur")},
			{Key: "SERVER_PASSWORD", Label: "Senha de entrada", Type: FieldPassword, Secret: true},
			{Key: "ADMIN_PASSWORD", Label: "Senha de admin", Type: FieldPassword, Secret: true, Required: true},
			{Key: "MAX_PLAYERS", Label: "Máximo de jogadores", Type: FieldInt, Default: "20", Min: ptr(1), Max: ptr(127)},
			{Key: "UPDATE_ON_START", Label: "Atualizar ao subir", Type: FieldBool, Default: "true", Advanced: true},
			{Key: "BACKUP_ON_STOP", Label: "Backup ao parar", Type: FieldBool, Default: "true", Advanced: true},
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
		ID:          "factoriotools/factorio",
		Game:        "factorio",
		GameLabel:   "Factorio",
		Short:       "FC",
		Image:       "factoriotools/factorio:stable",
		Description: "Servidor headless de Factorio, com mods e RCON.",
		Docs:        "https://github.com/factoriotools/factorio-docker",
		Ports: []Port{
			{Container: 34197, Protocol: "udp", DefaultHost: 34197, Label: "Jogo"},
			{Container: 27015, Protocol: "tcp", DefaultHost: 27015, Label: "RCON", Optional: true},
		},
		Volumes:          []Volume{{Host: "./data", Container: "/factorio", Data: true}},
		DefaultMemory:    "4g",
		MinMemory:        "1g",
		DefaultCPUs:      2,
		StopGraceSeconds: 60,
		Fields: []Field{
			{Key: "SAVE_NAME", Label: "Nome do save", Type: FieldText, Default: "gamedock",
				Help: "Se não existir, a imagem cria um mapa novo com esse nome."},
			{Key: "UPDATE_MODS_ON_START", Label: "Atualizar mods ao subir", Type: FieldBool, Default: "false"},
			{Key: "DLC_SPACE_AGE", Label: "DLC Space Age", Type: FieldBool, Default: "false"},
			{Key: "GENERATE_NEW_SAVE", Label: "Gerar save novo", Type: FieldBool, Default: "true", Advanced: true},
			{Key: "LOAD_LATEST_SAVE", Label: "Carregar o save mais novo", Type: FieldBool, Default: "true", Advanced: true},
		},
	},
	{
		ID:               "wolveix/satisfactory-server",
		Game:             "satisfactory",
		GameLabel:        "Satisfactory",
		Short:            "SF",
		Image:            "wolveix/satisfactory-server:latest",
		Description:      "Servidor dedicado de Satisfactory. Precisa de bastante RAM.",
		Docs:             "https://github.com/wolveix/satisfactory-server",
		Ports:            []Port{{Container: 7777, Protocol: "udp", DefaultHost: 7777, Label: "Jogo"}},
		Volumes:          []Volume{{Host: "./config", Container: "/config", Data: true}},
		DefaultMemory:    "12g",
		MinMemory:        "8g",
		DefaultCPUs:      4,
		StopGraceSeconds: 120,
		Fields: []Field{
			{Key: "MAXPLAYERS", Label: "Máximo de jogadores", Type: FieldInt, Default: "4", Min: ptr(1), Max: ptr(16)},
			{Key: "PGID", Label: "GID do processo", Type: FieldInt, Default: "1000", Advanced: true},
			{Key: "PUID", Label: "UID do processo", Type: FieldInt, Default: "1000", Advanced: true},
			{Key: "STEAMBETA", Label: "Branch experimental", Type: FieldBool, Default: "false"},
			{Key: "AUTOSAVENUM", Label: "Autosaves guardados", Type: FieldInt, Default: "5", Min: ptr(1), Max: ptr(50), Advanced: true},
			{Key: "MAXTICKRATE", Label: "Tickrate máximo", Type: FieldInt, Default: "30", Min: ptr(5), Max: ptr(120), Advanced: true},
			{Key: "MAXOBJECTS", Label: "Máximo de objetos", Type: FieldInt, Default: "2162688", Min: ptr(100000), Advanced: true},
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
