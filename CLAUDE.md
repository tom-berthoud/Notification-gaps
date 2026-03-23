# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GAPS Discord Bot — a Go application that scrapes student grades and absences from the HEIG-VD GAPS system and sends real-time notifications via Discord. Fork of `heig-lherman/gaps-cli` with added Discord bot, slash commands, automatic notifications, and cloud deployment.

All Discord-facing text is in **French**.

## Build & Run

```bash
# Build
go build -o gaps-cli .

# Run Discord bot (main mode)
./gaps-cli bot --log-level info

# Other CLI modes
./gaps-cli login
./gaps-cli grades
./gaps-cli absences
./gaps-cli scraper --interval 300
```

Docker: multi-stage Alpine build (`Dockerfile`). Deployment on Render.com (`deploy/render.yaml`).

No tests exist (`*_test.go` files are absent). No linter configured.

## Architecture

```
main.go          → cmd.Execute() entry point (Cobra CLI)
cmd/
  root.go        → Config via dual Viper instances (defaultViper + credentialsViper)
  bot.go         → Discord bot: polling loop, slash commands, grade diff, notifications (~1000 lines, core file)
  scraper.go     → Legacy webhook-based scraper mode
  login.go       → Auth to GAPS
  grades.go, absences.go, classes.go, report-card.go, version.go → CLI subcommands
gaps/            → HTTP client for GAPS API (cookie auth, AJAX calls)
parser/          → HTML parsing with goquery + regex for grades, absences, report cards
notifier/        → Discord webhook formatting (legacy, used by scraper mode)
cal/             → Swiss holiday calendar definitions
util/            → Error/logging helpers
```

## Key Design Patterns

- **Config**: Viper with `GAPS_` env prefix. Hyphens/dots → underscores (`GAPS_DISCORD_BOT_TOKEN`). Credentials in separate `~/.config/gaps-cli/credentials.yaml` (0600 perms).
- **Grade history**: JSON file diffing (`grades-history.json`). On each poll, fetches grades for all academic years, compares to stored snapshot, notifies on changes.
- **Semester logic**: S1-S6 mapped to years 1-3. Odd = autumn (Sept-Jan), even = spring (Feb-Aug). `GAPS_STUDY_START_YEAR` anchors the calculation.
- **Discord embeds**: Color-coded by grade (green ≥5, orange ≥4, red <4, grey=none). Max 10 embeds per message. All slash command responses are ephemeral.
- **Auth**: Cookie-based (`GAPSSESSID`), browser User-Agent spoofing, auto-refresh on expiry.
- **Health endpoint**: `/health` on port 8080 keeps Render.com free tier alive.

## Required Environment Variables (bot mode)

`GAPS_LOGIN_USERNAME`, `GAPS_LOGIN_PASSWORD`, `GAPS_DISCORD_BOT_TOKEN`, `GAPS_DISCORD_CHANNEL_ID`, `GAPS_DISCORD_GUILD_ID`, `GAPS_STUDY_START_YEAR`

## Future Code Quality Improvements

- **Découper `bot.go`** (~1000 lignes) en fichiers séparés : `bot_commands.go` (handlers), `bot_embeds.go` (builders d'embeds), `bot_scrape.go` (polling/diff de notes).
- **Ajouter des tests** — surtout pour `parser/grades.go` (parsing HTML) et la logique de diff de notes, facilement testables avec des fixtures HTML.
- **Notification d'absences** — surveiller les nouvelles absences dans le polling, pas seulement les notes.
- **Notifications par utilisateur** — permettre à chaque utilisateur de s'abonner via `/subscribe` au lieu de notifier tout un channel.
