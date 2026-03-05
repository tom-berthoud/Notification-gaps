# GAPS Discord Bot

> Bot Discord personnel pour suivre automatiquement ses notes et absences HEIG-VD.

## Comment ça fonctionne

### Mode en ligne (Render + UptimeRobot)


```text
┌───────────────────────────────────────────────────────────────────┐
│                        TOUTES LES 10 MIN                          │
│                                                                   │
│  UptimeRobot ── ping ──► Render (gaps-cli bot)                    │
│                               │                                   │
│                               ▼                                   │
│                          Se connecte à GAPS                       │
│                          (heig-vd.ch)                             │
│                               │                                   │
│                     Nouvelle note détectée ?                      │
│                         OUI │        NON                          │
│                             ▼         ▼                           │
│                Embed Discord envoyé   Rien (silence)              │
│                dans le salon          Log: "No changes"           │
└───────────────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────────┐
│                      À LA DEMANDE (toi)                           │
│                                                                   │
│  Toi (tel/PC) ── /notes ──► Render ──► GAPS ──► réponse éphémère  │
│                                                                   │
│  Réponse éphémère = visible uniquement par toi,                   │
│  disparaît quand tu fermes Discord, jamais dans le salon.         │
└───────────────────────────────────────────────────────────────────┘
```


### Ce qui est visible par qui ?

| Message | Visible par | Reste dans le salon |
|---------|------------|---------------------|
| Notif automatique nouvelle note | Tout le monde sur le serveur | ✅ Oui |
| Message de démarrage du bot | Tout le monde sur le serveur | ✅ Oui |
| Réponse à `/notes`, `/moyenne`, `/recap`... | **Toi uniquement** | ❌ Non (éphémère) |
| Réponse à `/clear` | **Toi uniquement** | ❌ Non (éphémère) |

> **Conseil :** crée un serveur Discord privé avec uniquement toi dedans. Les notifs automatiques resteront visibles dans le salon (utile pour les retrouver plus tard), et les commandes slash ne seront visibles que par toi de toute façon.

### À quoi sert `/clear` ?

Les commandes slash (`/notes`, etc.) sont éphémères — elles ne polluent pas le salon. En revanche, les **notifications automatiques** et le **message de démarrage** s'accumulent dans le salon au fil du temps. `/clear` supprime jusqu'à 100 de ces messages d'un coup.


## Fonctionnalités

### Notifications automatiques
Dès qu'une nouvelle note apparaît sur GAPS, le bot envoie un embed dans le salon :
- 🟢 Vert ≥ 5.0
- 🟡 Orange ≥ 4.0
- 🔴 Rouge < 4.0

Affiche : matière, nom de l'épreuve, ta note, la moyenne de classe, le poids.

### Commandes slash (réponses visibles uniquement par toi)

| Commande | Description |
|----------|-------------|
| `/notes` | Notes du semestre en cours |
| `/notes semestre:4` | Notes d'un semestre précis (S1 à S6) |
| `/allnotes` | Toutes les notes depuis le début |
| `/moyenne` | Moyennes générales par matière avec 🟢🟡🔴 |
| `/recap` | Résumé : nombre de notes, combien ≥5 / 4-5 / <4 |
| `/manquantes` | Notes attendues mais pas encore publiées |
| `/absences` | Absences par cours avec taux et seuils d'alerte |
| `/statut` | État du bot : dernière vérif, prochain check, nb de notes |
| `/clear` | Supprime les messages du salon (100 max) |


## Mapping des semestres

Le numéro de semestre correspond à ton parcours global depuis ta première rentrée :

| Commande | Année académique | Saison | Dates approximatives |
|----------|-----------------|--------|----------------------|
| `/notes semestre:1` | Année 1 (ex: 2024-2025) | Automne | Oct – Jan |
| `/notes semestre:2` | Année 1 (ex: 2024-2025) | Printemps | Fév – Juin |
| `/notes semestre:3` | Année 2 (ex: 2025-2026) | Automne | Oct – Jan |
| `/notes semestre:4` | Année 2 (ex: 2025-2026) | Printemps | Fév – Juin |
| `/notes semestre:5` | Année 3 | Automne | Oct – Jan |
| `/notes semestre:6` | Année 3 | Printemps | Fév – Juin |

Le tri par semestre est basé sur la **date de l'épreuve** : automne = sept–jan, printemps = fév–août.
Configure `GAPS_STUDY_START_YEAR` avec l'année de ta première rentrée (ex: `2024` pour une rentrée en septembre 2024).


## Installation

### Prérequis

- Un compte [Discord](https://discord.com) et un serveur Discord (même privé avec toi seul)
- Go 1.21+ (pour lancer en local)
- Un compte [Render](https://render.com) (pour le déploiement en ligne)


## Option A — Lancer en local

Utile pour tester avant de déployer.

### 1. Cloner et compiler

```bash
git clone https://github.com/tom-berthoud/Notification-gaps.git
cd Notification-gaps
go build -o gaps-cli .
```

### 2. Configurer les variables

```bash
cp .env.example .env
# Édite .env avec tes valeurs
set -a; source .env; set +a
```

### 3. Lancer

```bash
./gaps-cli bot --log-level info
```

Le bot démarre, se connecte à Discord et commence à vérifier les notes toutes les 10 minutes.

---

## Option B — Déploiement en ligne sur Render (recommandé)

Le bot tourne 24h/24 sans que ton ordinateur soit allumé.

### Étape 1 — Créer le bot Discord

1. Va sur [discord.com/developers](https://discord.com/developers/applications)
2. Clique **Nouvelle application** → donne un nom (ex: `GAPS Bot`)
3. Menu gauche → **Bot**
   - Clique **Réinitialiser le token** → copie-le → c'est `GAPS_DISCORD_BOT_TOKEN`
   - Active **Message Content Intent**
4. Menu gauche → **OAuth2** → **Générateur d'URL**
   - Coche les **champs d'application** : `bot` + `applications.commands`
   - Coche les **permissions** :
     - `Envoyer des messages`
     - `Intégrer des liens`
     - `Utiliser les commandes slash`
     - `Gérer les messages` (pour `/clear`)
   - Copie l'URL générée en bas
5. Ouvre l'URL → sélectionne ton serveur → **Autoriser**

> Le bot apparaît dans les membres de ton serveur (hors ligne pour l'instant).

### Étape 2 — Récupérer les identifiants Discord

Dans Discord : **Paramètres** → **Apparence** → **Mode développeur** → Activé

- Clic droit sur l'icône du serveur → **Copier l'identifiant** → `GAPS_DISCORD_GUILD_ID`
- Clic droit sur le salon de notifications → **Copier l'identifiant** → `GAPS_DISCORD_CHANNEL_ID`

### Étape 3 — Déployer sur Render

1. Crée un compte sur [render.com](https://render.com)
2. **New** → **Web Service** → connecte ton GitHub → sélectionne ce repo
3. Configure :

| Champ | Valeur |
|-------|--------|
| **Language** | Go |
| **Build Command** | `go build -o gaps-cli .` |
| **Start Command** | `./gaps-cli bot` |
| **Plan** | Free |

4. Onglet **Environment** → ajoute ces 6 variables :

| Variable | Description | Exemple |
|----------|-------------|---------|
| `GAPS_LOGIN_USERNAME` | Identifiant HEIG-VD | `prenom.nom` |
| `GAPS_LOGIN_PASSWORD` | Mot de passe HEIG-VD | `motdepasse` |
| `GAPS_DISCORD_BOT_TOKEN` | Token du bot | `MTQ3OTI...` |
| `GAPS_DISCORD_CHANNEL_ID` | ID du salon | `1234567890` |
| `GAPS_DISCORD_GUILD_ID` | ID du serveur | `9876543210` |
| `GAPS_STUDY_START_YEAR` | Année de première rentrée | `2024` |

5. Clique **Deploy** → attends que le build termine → le bot passe en ligne ✅

### Note — Perte d'historique au redémarrage (free tier)

Sur le free tier Render, l'historique des notes est stocké dans `/tmp` qui est effacé à chaque redémarrage. Si le bot redémarre au moment exact où une note apparaît sur GAPS, cette notification sera manquée. En pratique, avec UptimeRobot actif, les redémarrages sont rares et courts. Pour une persistance totale, Render propose des "Disks" (payants).

### Étape 4 — Éviter la mise en veille (free tier)

Le free tier Render endort le service après 15 min sans requête HTTP. Le bot expose un endpoint `/health` prévu pour ça.

1. Crée un compte sur [uptimerobot.com](https://uptimerobot.com) (gratuit)
2. **Add New Monitor** :
   - Type : **HTTP(S)**
   - URL : `https://ton-service.onrender.com/health`
   - Intervalle : **5 minutes**
3. Sauvegarde

UptimeRobot ping le bot toutes les 5 min → Render ne le met jamais en veille → le bot surveille GAPS en permanence.


## Crédits

Ce projet est un fork enrichi de [heig-lherman/gaps-cli](https://github.com/heig-lherman/gaps-cli), un outil CLI open-source pour interagir avec le système GAPS de la HEIG-VD. Le scraping GAPS, le parsing HTML et la logique d'authentification proviennent entièrement de ce projet original.

Les ajouts de ce fork :
- Bot Discord avec commandes slash
- Notifications automatiques via webhook Discord
- Déploiement cloud (Render)
- Filtrage par semestre

**Auteur du fork :** [Tom Berthoud](https://github.com/tom-berthoud)

Merci à Claude code pour avoir carry le projet.

N'hésitez pas à contribuer, signaler des bugs ou proposer des améliorations via les issues ou pull requests !
