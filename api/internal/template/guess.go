package template

import "strings"

// what is known about a container that did not come from a template
type Hints struct {
	Image   string
	Name    string
	Service string
	Labels  map[string]string
	Ports   []int
}

var imageNeedles = map[Category][]string{
	CategoryMedia: {
		"jellyfin", "plex", "emby", "sonarr", "radarr", "lidarr", "readarr",
		"bazarr", "prowlarr", "jackett", "qbittorrent",
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
		"flaresolverr", "squid", "socks5",
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

// ports only one kind of service tends to publish; 80, 443 and 8080 are left out on purpose
var knownPorts = map[int]Category{
	25565: CategoryGames, 25575: CategoryGames, 19132: CategoryGames,
	7777: CategoryGames, 2456: CategoryGames, 2457: CategoryGames,
	27015: CategoryGames, 8211: CategoryGames, 34197: CategoryGames,

	5432: CategoryDatabase, 3306: CategoryDatabase, 27017: CategoryDatabase,
	6379: CategoryDatabase, 9200: CategoryDatabase, 8086: CategoryDatabase,
	5984: CategoryDatabase, 9042: CategoryDatabase, 11211: CategoryDatabase,
	1433: CategoryDatabase,

	53: CategoryNetwork, 51820: CategoryNetwork, 1194: CategoryNetwork,
	3128: CategoryNetwork,

	8096: CategoryMedia, 32400: CategoryMedia, 8989: CategoryMedia,
	7878: CategoryMedia, 9696: CategoryMedia, 8686: CategoryMedia,
	8787: CategoryMedia, 6767: CategoryMedia, 9091: CategoryMedia,

	8123: CategoryUtilities, 9443: CategoryUtilities,
}

// words from homemade container names, matched whole so "bot" never hits "robot"
var homemadeWords = map[Category][]string{
	CategoryUtilities: {
		"bot", "webhook", "worker", "scraper", "crawler", "notifier",
		"exporter", "cron", "backup", "dashboard",
	},
}

// labels that describe the image; the rest is ignored, a container behind the proxy carries "traefik.*"
var describingLabels = []string{
	"org.opencontainers.image.title",
	"org.opencontainers.image.description",
	"org.opencontainers.image.source",
	"org.opencontainers.image.url",
	"org.opencontainers.image.documentation",
	"org.label-schema.name",
	"org.label-schema.description",
	"org.label-schema.url",
}

func GuessCategory(h Hints) (Category, bool) {
	if category, ok := bySubstring(h.Image); ok {
		return category, true
	}
	if category, ok := bySubstring(labelText(h.Labels)); ok {
		return category, true
	}
	if category, ok := byPort(h.Ports); ok {
		return category, true
	}
	if category, ok := bySubstring(h.Name + " " + h.Service); ok {
		return category, true
	}
	if category, ok := byWord(h.Image, h.Name, h.Service); ok {
		return category, true
	}
	return CategoryOther, false
}

func bySubstring(text string) (Category, bool) {
	text = strings.ToLower(text)
	if strings.TrimSpace(text) == "" {
		return CategoryOther, false
	}
	for _, category := range AllCategories {
		for _, needle := range imageNeedles[category] {
			if strings.Contains(text, needle) {
				return category, true
			}
		}
	}
	return CategoryOther, false
}

func byPort(ports []int) (Category, bool) {
	for _, p := range ports {
		if category, ok := knownPorts[p]; ok {
			return category, true
		}
	}
	return CategoryOther, false
}

func byWord(texts ...string) (Category, bool) {
	words := map[string]bool{}
	for _, text := range texts {
		for _, word := range strings.FieldsFunc(strings.ToLower(text), notAlphanumeric) {
			words[word] = true
		}
	}
	for _, category := range AllCategories {
		for _, word := range homemadeWords[category] {
			if words[word] {
				return category, true
			}
		}
	}
	return CategoryOther, false
}

func notAlphanumeric(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return false
	case r >= '0' && r <= '9':
		return false
	default:
		return true
	}
}

func labelText(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	var b strings.Builder
	for _, key := range describingLabels {
		if v := labels[key]; v != "" {
			b.WriteString(v)
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func (c *Catalog) CategoryFor(h Hints) Category {
	category, _ := GuessCategory(h)
	return category
}
