<div align="center">
  <h1>🐳 OkDock</h1>
  <p><b>A lightweight, resilient panel to manage and run Docker containers on your home server.</b></p>
  
  [![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](#)
  [![Angular](https://img.shields.io/badge/Angular-20-DD0031?logo=angular&logoColor=white)](#)
  [![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)](#)
</div>

<br />


**OkDock** is a management panel tailored for home servers. Instead of hijacking your containers using the Docker SDK, OkDock acts as a visual orchestrator: every instance becomes a directory with its own `docker-compose.yml`, and the panel runs `docker compose` on top of it. 

**The result? Total resilience.** If the panel ever goes down, your infrastructure remains completely manageable directly from your terminal.

## ✨ Features

- 🏂 **Kanban-style Dashboard:** Monitor your containers by state (stopped, provisioning, running, etc.) with real-time SSE updates.
- 🗂 **Smart Stacking:** Containers from the same compose project collapse into a single tile, keeping your board clean.
- 📦 **Template Engine:** Start new instances in seconds. Ships with built-in templates (games, media, databases, network) and allows custom ones.
- 🧠 **RAM Budget Management:** OkDock calculates the memory cap of running instances before spinning up new ones, preventing OOM errors.
- 🌍 **DuckDNS Integration:** Easily link instances to your `duckdns.org` subdomains. The panel keeps your IPs automatically updated.
- 🔍 **Live Console & Compose Viewer:** Read logs in real-time and inspect the actual `docker-compose.yml` generated on disk.
- 🤝 **Adopts Existing Containers:** Already running containers automatically show up on the board and can be managed seamlessly.
- 🔒 **Secure by Default:** Passwords never go into the compose file; they are securely stored in a `.env` file (`0600` permissions).

---

## 🚀 Getting Started (Server Installation)

Running OkDock on your server is as simple as running a single Docker Compose command. The panel carries its own `docker compose` binary (Alpine plugin), so it doesn't even depend on the host's version.

Create a `docker-compose.yml` file and run:

```bash
docker compose up -d
