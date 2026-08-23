package template

import "strings"

// palpites liga um pedaco do nome da imagem a uma categoria, e um imagePattern manda mais
var palpites = map[Category][]string{
	CategoryMedia: {
		"jellyfin", "plex", "emby", "sonarr", "radarr", "lidarr", "readarr",
		"bazarr", "prowlarr", "jackett", "flaresolverr", "qbittorrent",
		"transmission", "deluge", "sabnzbd", "nzbget", "navidrome", "airsonic",
		"audiobookshelf", "calibre", "immich", "photoprism", "tautulli",
		"overseerr", "jellyseerr", "komga", "kavita",
	},
	CategoryDatabase: {
		"postgres", "mysql", "mariadb", "mongo", "redis", "valkey", "influxdb",
		"clickhouse", "elasticsearch", "opensearch", "memcached", "couchdb",
		"cassandra", "timescale", "pgadmin", "adminer",
	},
	CategoryNetwork: {
		"nginx", "traefik", "caddy", "haproxy", "pihole", "pi-hole", "adguard",
		"wireguard", "openvpn", "duckdns", "ddns", "cloudflared", "tailscale",
		"unbound", "bind9", "dnsmasq", "swag", "authelia", "authentik",
	},
	CategoryGames: {
		"minecraft", "terraria", "valheim", "palworld", "factorio", "ark",
		"satisfactory", "rust-server", "steamcmd", "csgo", "cs2", "pufferpanel",
	},
	CategoryUtilities: {
		"portainer", "watchtower", "homeassistant", "home-assistant", "syncthing",
		"vaultwarden", "bitwarden", "uptime-kuma", "grafana", "prometheus",
		"gitea", "forgejo", "jenkins", "n8n", "homarr", "heimdall", "dashy",
		"filebrowser", "duplicati", "restic", "nextcloud", "owncloud", "samba",
		"paperless", "wikijs", "mealie", "speedtest",
	},
}

// GuessCategory adivinha pelo nome da imagem e devolve false quando nao reconhece nada
func GuessCategory(image string) (Category, bool) {
	image = strings.ToLower(image)
	// a ordem do mapa e aleatoria em Go, entao a varredura segue a ordem fixa das categorias
	for _, category := range AllCategories {
		for _, needle := range palpites[category] {
			if strings.Contains(image, needle) {
				return category, true
			}
		}
	}
	return CategoryOther, false
}

// CategoryForImage responde a categoria: o template manda, o palpite pelo nome vem depois
func (c *Catalog) CategoryForImage(image string) Category {
	if t, ok := c.TemplateForImage(image); ok {
		return t.Category
	}
	category, _ := GuessCategory(image)
	return category
}
