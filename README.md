# GAPS Discord Bot

Bot Discord pour suivre tes notes et absences HEIG-VD automatiquement.

- Vérifie les nouvelles notes toutes les **10 minutes** et envoie une notification dans un salon Discord
- Répond à des **commandes slash** pour consulter notes, moyennes et absences depuis n'importe où (PC, téléphone...)

---

## Fonctionnalités

### Notifications automatiques
Dès qu'une nouvelle note apparaît sur GAPS, le bot envoie un message dans ton salon :
- 🟢 Vert ≥ 5.0
- 🟡 Orange ≥ 4.0
- 🔴 Rouge < 4.0

### Commandes slash

| Commande | Description |
|----------|-------------|
| `/notes` | Notes du semestre en cours |
| `/notes semestre:3` | Notes d'un semestre précis (S1 à S6) |
| `/allnotes` | Toutes les notes (toutes les années) |
| `/moyenne` | Moyennes générales par matière |
| `/recap` | Résumé global : nombre de notes, statut, moyennes |
| `/manquantes` | Notes pas encore publiées |
| `/absences` | Absences par cours avec taux |
| `/clear` | Supprime les messages du salon (100 max) |

---

## Installation

### Étape 1 — Créer le bot Discord

1. Va sur [discord.com/developers](https://discord.com/developers/applications)
2. Clique sur **Nouvelle application** (en haut à droite)
3. Donne un nom à ton bot (ex: `GAPS Bot`) et valide

4. Dans le menu de gauche, clique sur **Bot**
   - Clique sur **Réinitialiser le token** et copie-le → c'est ton `GAPS_DISCORD_BOT_TOKEN`
   - Active **Message Content Intent** (plus bas sur la page)

5. Dans le menu de gauche, clique sur **OAuth2** puis **Générateur d'URL**
   - Dans **Champs d'application**, coche : `bot` et `applications.commands`
   - Dans **Permissions du bot**, coche :
     - `Envoyer des messages`
     - `Intégrer des liens`
     - `Utiliser les commandes slash`
     - `Gérer les messages` (nécessaire pour `/clear`)
   - Copie l'URL générée en bas de page

6. Colle l'URL dans ton navigateur → sélectionne ton serveur Discord → clique **Autoriser**

> Le bot apparaît maintenant dans la liste des membres de ton serveur (hors ligne pour l'instant).

---

### Étape 2 — Récupérer les identifiants Discord

Dans Discord, active le **mode développeur** :
- Paramètres utilisateur → Apparence → Mode développeur → **Activé**

Ensuite :
- **Clic droit sur ton serveur** (l'icône en haut à gauche) → **Copier l'identifiant du serveur** → c'est ton `GAPS_DISCORD_GUILD_ID`
- **Clic droit sur le salon** où tu veux recevoir les notifications → **Copier l'identifiant du salon** → c'est ton `GAPS_DISCORD_CHANNEL_ID`

---

### Étape 3 — Variables d'environnement

Copie `.env.example` en `.env` et remplis les valeurs :

```bash
cp .env.example .env
```

| Variable | Description | Exemple |
|----------|-------------|---------|
| `GAPS_LOGIN_USERNAME` | Ton identifiant HEIG-VD | `prenom.nom` |
| `GAPS_LOGIN_PASSWORD` | Ton mot de passe HEIG-VD | `motdepasse` |
| `GAPS_DISCORD_BOT_TOKEN` | Token du bot (onglet Bot) | `MTQ3OTI...` |
| `GAPS_DISCORD_CHANNEL_ID` | ID du salon de notifications | `1234567890` |
| `GAPS_DISCORD_GUILD_ID` | ID du serveur Discord | `9876543210` |
| `GAPS_STUDY_START_YEAR` | Année de ta première rentrée | `2024` |

---

### Étape 4 — Lancer le bot

**En local :**

```bash
git clone https://github.com/<ton-user>/Notification-gaps.git
cd Notification-gaps
go build -o gaps-cli .

set -a; source .env; set +a
./gaps-cli bot --log-level info
```

Le bot se connecte à Discord, enregistre les commandes slash et commence à vérifier les notes toutes les 10 minutes.

---

## Déploiement sur Render (gratuit, recommandé)

Render permet de faire tourner le bot 24h/24 sans serveur.

### Étape 1 — Connecter le repo

1. Crée un compte sur [render.com](https://render.com)
2. **New** → **Web Service**
3. Connecte ton compte GitHub et sélectionne ce repo

### Étape 2 — Configurer le service

| Champ | Valeur |
|-------|--------|
| **Language** | Go |
| **Build Command** | `go build -o gaps-cli .` |
| **Start Command** | `./gaps-cli bot` |
| **Plan** | Free |

### Étape 3 — Ajouter les variables d'environnement

Dans l'onglet **Environment**, ajoute les 6 variables du tableau ci-dessus.

### Étape 4 — Déployer

Clique sur **Deploy**. Une fois le build terminé, le bot passe en ligne sur Discord.

### Étape 5 — Éviter la mise en veille (free tier)

Le free tier Render met le service en veille après 15 min sans requête HTTP.
Pour maintenir le bot actif, utilise [UptimeRobot](https://uptimerobot.com) (gratuit) :

1. Crée un compte sur UptimeRobot
2. **Add New Monitor** → type **HTTP(S)**
3. URL : `https://ton-service.onrender.com/health`
4. Intervalle : **5 minutes**
5. Sauvegarde

Le bot restera en ligne en permanence.

---

## Mapping des semestres

Le numéro de semestre dans les commandes correspond à ton parcours global (S1 = premier semestre depuis ta rentrée) :

| Commande | Année académique | Saison |
|----------|-----------------|--------|
| `/notes semestre:1` | Année 1 (ex: 2024-2025) | Automne (oct–jan) |
| `/notes semestre:2` | Année 1 (ex: 2024-2025) | Printemps (fév–juin) |
| `/notes semestre:3` | Année 2 (ex: 2025-2026) | Automne (oct–jan) |
| `/notes semestre:4` | Année 2 (ex: 2025-2026) | Printemps (fév–juin) |
| `/notes semestre:5` | Année 3 | Automne (oct–jan) |
| `/notes semestre:6` | Année 3 | Printemps (fév–juin) |

> Configure `GAPS_STUDY_START_YEAR` avec l'année de ta première rentrée.
> Ex: si tu as commencé en septembre 2024, mets `2024`.

---

## Basé sur

[heig-lherman/gaps-cli](https://github.com/heig-lherman/gaps-cli)
