# 🚀 gaps-cli – Version modifiée (Envoi vers Discord)

> ✅ **Projet en cours de développement**

Cette version est basée sur le projet original :
🔗 <https://github.com/heig-lherman/gaps-cli>

Elle ajoute une fonctionnalité :
✅ **Envoi automatique des résultats GAPS vers un serveur Discord** via un webhook.

L’objectif est de récupérer les notes, les formater proprement, et les publier dans un salon Discord depuis une application en **Go**.

---

## ✅ Fonctionnalités

- Récupération des notes GAPs
- Affichage console
- Envoi automatique sur Discord (webhook)
- Message formaté par matière / note / moyenne
- Identifiants stockés dans `.env`

### 🔧 Fonctionnalités en cours de développement

- Embeds Discord plus visuels
- Version Docker
- Multi webhooks pour plusieurs serveurs
_différents type de webhooks (canal texte, annonces, etc.)_

---

## 📦 Installation

Cloner le projet :

```bash
git clone https://github.com/<ton-user>/gaps-cli-discord.git
cd gaps-cli-discord
go build -o gaps-cli .
```

Créer un fichier `.env` (voir `.env.example`) et exporter les variables :

```bash
set -a; source .env; set +a
```

Lancer le scraper (ex: toutes les 5 minutes) :

```bash
./gaps-cli scraper --interval 300
```

## 🖥️ Tourner 24/7 (H24)

Deux options simples :

1) **Systemd** (Linux) : copier `deploy/gaps-cli-scraper.service` dans `/etc/systemd/system/`, créer `/etc/gaps-cli.env` avec les variables, puis :

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gaps-cli-scraper
```

2) **Docker** : (persist l’historique des notes via un volume)

```bash
docker build -t gaps-cli .
docker run --rm -it \
  --env-file .env \
  -v "$PWD/history:/history" \
  gaps-cli scraper --interval 300
```

## 🇨🇭 Hébergement en Suisse ?

- Si tu veux surtout que **tes identifiants AAI + l’historique** restent en Suisse, oui: prends un VPS/serveur en Suisse.
- Si tu envoies les notes sur **Discord**, elles sortent de toute façon de Suisse (Discord). L’hébergement en Suisse protège surtout la partie “scraper + stockage local”.
