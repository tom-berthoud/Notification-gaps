# GAPS Discord Bot

Bot Discord pour suivre tes notes et absences HEIG-VD automatiquement.

- Vérifie les nouvelles notes toutes les **10 minutes** et envoie une notification dans un channel Discord
- Répond à des **slash commands** pour consulter notes, moyennes et absences depuis n'importe où

---

## Fonctionnalités

### Notifications automatiques
Dès qu'une nouvelle note apparaît sur GAPS, le bot envoie un embed coloré dans ton channel :
- 🟢 Vert ≥ 5.0
- 🟡 Orange ≥ 4.0
- 🔴 Rouge < 4.0

### Slash commands

| Commande | Description |
|----------|-------------|
| `/notes` | Notes du semestre en cours |
| `/notes semestre:3` | Notes d'un semestre précis (S1 à S6) |
| `/allnotes` | Toutes les notes (toutes les années) |
| `/moyenne` | Moyennes générales par matière |
| `/recap` | Résumé global : nb de notes, statut, moyennes |
| `/manquantes` | Notes pas encore publiées |
| `/absences` | Absences par cours avec taux |
| `/clear` | Supprime les messages du canal (100 max) |

---

## Installation

### 1. Créer le bot Discord

1. Va sur [discord.com/developers](https://discord.com/developers/applications) → **New Application**
2. Onglet **Bot** → **Reset Token** → copie le token
3. Active **Message Content Intent** dans l'onglet Bot
4. Onglet **OAuth2** → **URL Generator** :
   - Scopes : `bot` + `applications.commands`
   - Permissions : `Send Messages` + `Embed Links` + `Use Slash Commands` + `Manage Messages`
5. Colle l'URL générée dans le navigateur et invite le bot sur ton serveur

### 2. Récupérer les IDs Discord

Active le **mode développeur** (Paramètres Discord → Apparence → Mode développeur), puis :
- Clic droit sur ton serveur → **Copier l'identifiant** → `GAPS_DISCORD_GUILD_ID`
- Clic droit sur le channel de notifications → **Copier l'identifiant** → `GAPS_DISCORD_CHANNEL_ID`

### 3. Variables d'environnement

| Variable | Description |
|----------|-------------|
| `GAPS_LOGIN_USERNAME` | Ton username HEIG-VD (ex: `prenom.nom`) |
| `GAPS_LOGIN_PASSWORD` | Ton mot de passe HEIG-VD |
| `GAPS_DISCORD_BOT_TOKEN` | Token du bot Discord |
| `GAPS_DISCORD_CHANNEL_ID` | ID du channel pour les notifications |
| `GAPS_DISCORD_GUILD_ID` | ID du serveur Discord |
| `GAPS_STUDY_START_YEAR` | Année de début de tes études (ex: `2024`) |

### 4. Lancer en local

```bash
git clone https://github.com/<ton-user>/Notification-gaps.git
cd Notification-gaps
go build -o gaps-cli .

export GAPS_LOGIN_USERNAME=prenom.nom
export GAPS_LOGIN_PASSWORD=motdepasse
export GAPS_DISCORD_BOT_TOKEN=token_du_bot
export GAPS_DISCORD_CHANNEL_ID=id_channel
export GAPS_DISCORD_GUILD_ID=id_serveur
export GAPS_STUDY_START_YEAR=2024

./gaps-cli bot --log-level info
```

---

## Déploiement sur Render (gratuit)

1. Fork ce repo et connecte-le sur [render.com](https://render.com)
2. **New Web Service** → sélectionne le repo
3. Configure :
   - **Language** : Go
   - **Build Command** : `go build -o gaps-cli .`
   - **Start Command** : `./gaps-cli bot`
4. Ajoute les 6 variables d'environnement dans l'onglet **Environment**
5. Deploy

### Éviter le sleep (free tier)

Le free tier Render endort le service après 15 min sans requête HTTP.
Pour maintenir le bot actif, utilise [UptimeRobot](https://uptimerobot.com) (gratuit) :
- New Monitor → HTTP(S)
- URL : `https://ton-service.onrender.com/health`
- Intervalle : 5 minutes

---

## Mapping semestres

Le numéro de semestre dans les commandes correspond à ton parcours global :

| Commande | Année académique | Saison |
|----------|-----------------|--------|
| `semestre:1` | Année 1 (ex: 2024-2025) | Automne |
| `semestre:2` | Année 1 (ex: 2024-2025) | Printemps |
| `semestre:3` | Année 2 (ex: 2025-2026) | Automne |
| `semestre:4` | Année 2 (ex: 2025-2026) | Printemps |
| `semestre:5` | Année 3 | Automne |
| `semestre:6` | Année 3 | Printemps |

Configure `GAPS_STUDY_START_YEAR` avec l'année de ta première rentrée (ex: `2024` si tu as commencé en septembre 2024).

---

## Basé sur

[heig-lherman/gaps-cli](https://github.com/heig-lherman/gaps-cli) — merci pour le travail de base.
