package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/r3labs/diff/v3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"lutonite.dev/gaps-cli/gaps"
	"lutonite.dev/gaps-cli/parser"
)

type BotCommand struct {
	interval    int
	historyFile string
}

var (
	botOpts = &BotCommand{}
	botCmd  = &cobra.Command{
		Use:   "bot",
		Short: "Runs a Discord bot that checks grades and responds to slash commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			token := credentialsViper.GetString(DiscordBotTokenViperKey.Key())
			channelId := defaultViper.GetString(DiscordChannelIdViperKey.Key())
			guildId := defaultViper.GetString(DiscordGuildIdViperKey.Key())

			if token == "" {
				return fmt.Errorf("Discord bot token is required (--discord-token or GAPS_DISCORD_BOT_TOKEN)")
			}
			if channelId == "" {
				return fmt.Errorf("Discord channel ID is required (--discord-channel or GAPS_DISCORD_CHANNEL_ID)")
			}

			log.Info("Refreshing GAPS token")
			refreshToken(
				defaultViper.GetString(UsernameViperKey.Key()),
				credentialsViper.GetString(PasswordViperKey.Key()),
			)

			dg, err := discordgo.New("Bot " + token)
			if err != nil {
				return fmt.Errorf("failed to create Discord session: %w", err)
			}

			dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
				log.Infof("Bot connected as %s", r.User.String())
			})

			dg.AddHandler(botOpts.handleInteraction(channelId))

			if err := dg.Open(); err != nil {
				return fmt.Errorf("failed to open Discord connection: %w", err)
			}
			defer dg.Close()

			if err := botOpts.registerSlashCommands(dg, guildId); err != nil {
				return fmt.Errorf("failed to register slash commands: %w", err)
			}

			// HTTP health endpoint for Render.com keep-alive
			go func() {
				http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("ok"))
				})
				port := os.Getenv("PORT")
				if port == "" {
					port = "8080"
				}
				log.Infof("Health server listening on :%s", port)
				if err := http.ListenAndServe(":"+port, nil); err != nil {
					log.WithError(err).Error("Health server error")
				}
			}()

			// Initial scrape
			if err := botOpts.runScrape(dg, channelId); err != nil {
				log.WithError(err).Error("Initial scrape failed")
			}

			ticker := time.NewTicker(time.Duration(botOpts.interval) * time.Second)
			defer ticker.Stop()

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)

			log.Infof("Bot running, checking grades every %ds", botOpts.interval)
			for {
				select {
				case <-c:
					log.Info("Shutting down bot")
					return nil
				case <-ticker.C:
					if err := botOpts.runScrape(dg, channelId); err != nil {
						log.WithError(err).Error("Scrape failed")
					}
				}
			}
		},
	}
)

func init() {
	botCmd.Flags().StringP(UsernameViperKey.Flag(), "u", "", "einet aai username")
	defaultViper.BindPFlag(UsernameViperKey.Key(), botCmd.Flags().Lookup(UsernameViperKey.Flag()))

	botCmd.Flags().StringP(PasswordViperKey.Flag(), "p", "", "einet aai password")
	credentialsViper.BindPFlag(PasswordViperKey.Key(), botCmd.Flags().Lookup(PasswordViperKey.Flag()))

	botCmd.Flags().StringVar(&botOpts.historyFile, GradesHistoryFileViperKey.Flag(), "", "history file")
	defaultViper.BindPFlag(GradesHistoryFileViperKey.Key(), botCmd.Flags().Lookup(GradesHistoryFileViperKey.Flag()))
	defaultViper.SetDefault(GradesHistoryFileViperKey.Key(), getConfigDirectory()+"/gaps-cli/grades-history.json")

	botCmd.Flags().String(DiscordBotTokenViperKey.Flag(), "", "Discord bot token")
	credentialsViper.BindPFlag(DiscordBotTokenViperKey.Key(), botCmd.Flags().Lookup(DiscordBotTokenViperKey.Flag()))

	botCmd.Flags().String(DiscordChannelIdViperKey.Flag(), "", "Discord channel ID for grade notifications")
	defaultViper.BindPFlag(DiscordChannelIdViperKey.Key(), botCmd.Flags().Lookup(DiscordChannelIdViperKey.Flag()))

	botCmd.Flags().String(DiscordGuildIdViperKey.Flag(), "", "Discord guild (server) ID")
	defaultViper.BindPFlag(DiscordGuildIdViperKey.Key(), botCmd.Flags().Lookup(DiscordGuildIdViperKey.Flag()))

	botCmd.Flags().IntVar(&botOpts.interval, "interval", 600, "Seconds between each grade check")

	botCmd.Flags().Uint(StudyStartYearViperKey.Flag(), 0, "Academic year you started your studies (e.g. 2023)")
	defaultViper.BindPFlag(StudyStartYearViperKey.Key(), botCmd.Flags().Lookup(StudyStartYearViperKey.Flag()))
	defaultViper.SetDefault(StudyStartYearViperKey.Key(), currentAcademicYear()-1)

	rootCmd.AddCommand(botCmd)
}

// semesterToYear maps a semester number (1-6) to its academic year.
func semesterToYear(sem int) uint {
	startYear := defaultViper.GetUint(StudyStartYearViperKey.Key())
	return startYear + uint((sem-1)/2)
}

// isAutumnGrade returns true if the grade date falls in the autumn semester (Sept–Jan).
// Zero dates are included in both semesters (shown always).
func isAutumnGrade(d time.Time) bool {
	if d.IsZero() {
		return true
	}
	m := d.Month()
	return m >= 9 || m <= 1
}

// isSpringGrade returns true if the grade date falls in the spring semester (Feb–Aug).
func isSpringGrade(d time.Time) bool {
	if d.IsZero() {
		return true
	}
	m := d.Month()
	return m >= 2 && m <= 8
}

// semesterFilter returns the date filter function for odd (autumn) or even (spring) semesters.
// Returns nil if no filter should be applied.
func semesterFilter(sem int) func(time.Time) bool {
	if sem%2 == 1 {
		return isAutumnGrade
	}
	return isSpringGrade
}

// registerSlashCommands registers slash commands on the guild (instant) or globally (up to 1h delay).
func (b *BotCommand) registerSlashCommands(dg *discordgo.Session, guildId string) error {
	semChoices := []*discordgo.ApplicationCommandOptionChoice{
		{Name: "S1", Value: 1},
		{Name: "S2", Value: 2},
		{Name: "S3", Value: 3},
		{Name: "S4", Value: 4},
		{Name: "S5", Value: 5},
		{Name: "S6", Value: 6},
	}

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "notes",
			Description: "Affiche tes notes (semestre en cours par défaut)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "semestre",
					Description: "Numéro de semestre (S1 à S6)",
					Required:    false,
					Choices:     semChoices,
				},
			},
		},
		{
			Name:        "allnotes",
			Description: "Affiche toutes tes notes (toutes les années)",
		},
		{
			Name:        "moyenne",
			Description: "Affiche les moyennes générales par matière",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "semestre",
					Description: "Numéro de semestre (S1 à S6)",
					Required:    false,
					Choices:     semChoices,
				},
			},
		},
		{
			Name:        "recap",
			Description: "Résumé rapide : nombre de notes, moyennes, statut global",
		},
		{
			Name:        "manquantes",
			Description: "Liste les notes pas encore publiées",
		},
		{
			Name:        "absences",
			Description: "Affiche tes absences",
		},
		{
			Name:        "clear",
			Description: "Supprime les messages du canal (100 max)",
		},
	}

	for _, cmd := range commands {
		if _, err := dg.ApplicationCommandCreate(dg.State.User.ID, guildId, cmd); err != nil {
			return fmt.Errorf("cannot create command %s: %w", cmd.Name, err)
		}
		log.Infof("Slash command /%s registered", cmd.Name)
	}
	return nil
}

// handleInteraction returns a handler for slash commands.
func (b *BotCommand) handleInteraction(channelId string) func(*discordgo.Session, *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		data := i.ApplicationCommandData()

		// /clear is handled separately (ephemeral + no embed)
		if data.Name == "clear" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
			})
			b.handleClear(s, i, channelId)
			return
		}

		// Acknowledge immediately (Discord requires response within 3s)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		var embeds []*discordgo.MessageEmbed
		var err error

		switch data.Name {
		case "notes":
			year := currentAcademicYear()
			var filter func(time.Time) bool
			for _, opt := range data.Options {
				if opt.Name == "semestre" {
					sem := int(opt.IntValue())
					year = semesterToYear(sem)
					filter = semesterFilter(sem)
				}
			}
			embeds, err = b.buildGradesEmbeds(year, filter)
		case "allnotes":
			embeds, err = b.buildAllGradesEmbeds()
		case "moyenne":
			year := currentAcademicYear()
			var filter func(time.Time) bool
			for _, opt := range data.Options {
				if opt.Name == "semestre" {
					sem := int(opt.IntValue())
					year = semesterToYear(sem)
					filter = semesterFilter(sem)
				}
			}
			embeds, err = b.buildMoyenneEmbeds(year, filter)
		case "recap":
			embeds, err = b.buildRecapEmbeds()
		case "manquantes":
			embeds, err = b.buildManquantesEmbeds()
		case "absences":
			embeds, err = b.buildAbsencesEmbeds()
		default:
			return
		}

		if err != nil {
			log.WithError(err).Error("Failed to fetch data for slash command")
			s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "❌ Erreur lors de la récupération des données GAPS.",
			})
			return
		}

		// Discord allows max 10 embeds per message
		for start := 0; start < len(embeds); start += 10 {
			end := start + 10
			if end > len(embeds) {
				end = len(embeds)
			}
			s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Embeds: embeds[start:end],
			})
		}
	}
}

// runScrape checks for new grades and sends a notification if any changed.
func (b *BotCommand) runScrape(dg *discordgo.Session, channelId string) error {
	if isTokenExpired() {
		refreshToken(
			defaultViper.GetString(UsernameViperKey.Key()),
			credentialsViper.GetString(PasswordViperKey.Key()),
		)
	}

	cfg := buildTokenClientConfiguration()
	year := currentAcademicYear()

	ga := gaps.NewGradesAction(cfg, year)
	grades, err := ga.FetchGrades()
	if err != nil {
		return fmt.Errorf("fetch grades: %w", err)
	}

	current := mapBotGrades(grades)
	previous, err := b.readHistory()
	if previous == nil {
		if err != nil {
			return err
		}
		log.Info("No history found, saving initial state")
		return b.writeHistory(current)
	}

	changes, _ := diff.Diff(previous, current, diff.TagName("diff"), diff.DisableStructValues())
	if len(changes) == 0 {
		log.Info("No grade changes")
		return nil
	}

	seen := make(map[string]bool)
	for _, change := range changes {
		g := resolveBotGrade(current, change)
		if g == nil || g.ClassMean == "-" {
			continue
		}
		key := g.Course + "|" + g.Description
		if seen[key] {
			continue
		}
		seen[key] = true

		embed := buildGradeNotifEmbed(g, resolveBotGrade(previous, change))
		dg.ChannelMessageSendEmbed(channelId, embed)
		log.Infof("Notified new grade: [%s] %s = %s", g.Course, g.Description, g.Grade)
	}

	return b.writeHistory(current)
}

// ── Grade helpers ────────────────────────────────────────────────────────────

type botGrade struct {
	Course      string    `json:"course" diff:"-"`
	Description string    `json:"description" diff:"Description,identifier"`
	Date        time.Time `json:"date" diff:"-"`
	Weight      float32   `json:"weight" diff:"-"`
	Grade       string    `json:"grade"`
	ClassMean   string    `json:"classMean"`
}

type botHistory map[string]map[string]*botGrade

func mapBotGrades(classes []*parser.ClassGrades) botHistory {
	result := make(botHistory)
	for _, class := range classes {
		result[class.Name] = make(map[string]*botGrade)
		for _, group := range class.GradeGroups {
			for _, g := range group.Grades {
				result[class.Name][g.Description] = &botGrade{
					Course:      class.Name,
					Description: g.Description,
					Date:        g.Date,
					Weight:      g.Weight,
					Grade:       g.Grade,
					ClassMean:   g.ClassMean,
				}
			}
		}
	}
	return result
}

func resolveBotGrade(h botHistory, change diff.Change) *botGrade {
	if change.Type == diff.DELETE || len(change.Path) < 2 {
		return nil
	}
	course, ok := h[change.Path[0]]
	if !ok {
		return nil
	}
	return course[change.Path[1]]
}

func (b *BotCommand) readHistory() (botHistory, error) {
	b.historyFile = defaultViper.GetString(GradesHistoryFileViperKey.Key())
	data, err := os.ReadFile(b.historyFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var h botHistory
	return h, json.Unmarshal(data, &h)
}

func (b *BotCommand) writeHistory(h botHistory) error {
	b.historyFile = defaultViper.GetString(GradesHistoryFileViperKey.Key())
	data, err := json.MarshalIndent(h, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(b.historyFile, data, 0o644)
}

// ── Embed builders ───────────────────────────────────────────────────────────

func gradeColor(grade string) int {
	v, err := strconv.ParseFloat(grade, 32)
	if err != nil {
		return 0x95a5a6 // grey — no grade yet
	}
	switch {
	case v >= 5.0:
		return 0x2ecc71 // green
	case v >= 4.0:
		return 0xf39c12 // orange
	default:
		return 0xe74c3c // red
	}
}

func buildGradeNotifEmbed(g *botGrade, prev *botGrade) *discordgo.MessageEmbed {
	title := fmt.Sprintf("📢 Nouvelle note — %s", g.Course)
	desc := fmt.Sprintf("**%s**\nNote : **%s** | Moy. classe : **%s**", g.Description, g.Grade, g.ClassMean)
	if prev != nil && prev.Grade != g.Grade {
		desc += fmt.Sprintf("\n*(ancienne note : %s)*", prev.Grade)
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		Color:       gradeColor(g.Grade),
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Poids : %.0f%%", g.Weight)},
		Timestamp:   g.Date.Format(time.RFC3339),
	}
}

func (b *BotCommand) buildAllGradesEmbeds() ([]*discordgo.MessageEmbed, error) {
	startYear := defaultViper.GetUint(StudyStartYearViperKey.Key())
	var embeds []*discordgo.MessageEmbed
	for year := startYear; year <= currentAcademicYear(); year++ {
		e, err := b.buildGradesEmbeds(year, nil)
		if err != nil {
			return nil, err
		}
		embeds = append(embeds, e...)
	}
	return embeds, nil
}

func (b *BotCommand) buildGradesEmbeds(year uint, dateFilter func(time.Time) bool) ([]*discordgo.MessageEmbed, error) {
	if isTokenExpired() {
		refreshToken(
			defaultViper.GetString(UsernameViperKey.Key()),
			credentialsViper.GetString(PasswordViperKey.Key()),
		)
	}
	cfg := buildTokenClientConfiguration()
	grades, err := gaps.NewGradesAction(cfg, year).FetchGrades()
	if err != nil {
		return nil, err
	}

	var embeds []*discordgo.MessageEmbed
	for _, class := range grades {
		fields := []*discordgo.MessageEmbedField{}
		for _, group := range class.GradeGroups {
			gradeCount := 0
			for _, g := range group.Grades {
				if dateFilter != nil && !dateFilter(g.Date) {
					continue
				}
				gradeCount++
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   g.Description,
					Value:  fmt.Sprintf("Note: **%s** | Moy: **%s** | Poids: %.0f%%", g.Grade, g.ClassMean, g.Weight),
					Inline: false,
				})
			}
			if gradeCount > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("— Moyenne %s", group.Name),
					Value:  fmt.Sprintf("**%s** (poids %d%%)", group.Mean, group.Weight),
					Inline: false,
				})
			}
		}
		if len(fields) == 0 {
			continue // skip courses with no grades in this semester
		}

		title := fmt.Sprintf("[%d-%d] %s", year, year+1, class.Name)
		if class.HasExam {
			title += " (examen)"
		}
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title:       title,
			Description: fmt.Sprintf("Moyenne générale : **%s**", class.GlobalMean),
			Color:       gradeColor(class.GlobalMean),
			Fields:      fields,
		})
	}

	if len(embeds) == 0 {
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("Notes %d-%d", year, year+1),
			Description: "Aucune note trouvée pour cette année.",
			Color:       0x95a5a6,
		})
	}
	return embeds, nil
}

func (b *BotCommand) buildAbsencesEmbeds() ([]*discordgo.MessageEmbed, error) {
	if isTokenExpired() {
		refreshToken(
			defaultViper.GetString(UsernameViperKey.Key()),
			credentialsViper.GetString(PasswordViperKey.Key()),
		)
	}
	cfg := buildTokenClientConfiguration()
	report, err := gaps.NewAbsencesAction(cfg, currentAcademicYear()).FetchAbsences()
	if err != nil {
		return nil, err
	}

	fields := []*discordgo.MessageEmbedField{}
	for _, course := range report.Courses {
		unjustified := course.Total - course.Justified
		if unjustified == 0 {
			continue
		}
		var rate float64
		if course.AbsolutePeriods > 0 {
			rate = float64(unjustified) / float64(course.AbsolutePeriods) * 100
		}
		emoji := "🟢"
		if rate >= 15 {
			emoji = "🔴"
		} else if rate >= 8 {
			emoji = "🟡"
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   course.Name,
			Value:  fmt.Sprintf("%s %d absences non-justifiées (%.1f%%)", emoji, unjustified, rate),
			Inline: false,
		})
	}

	desc := fmt.Sprintf("Étudiant : %s", report.Student)
	if len(fields) == 0 {
		desc += "\n\n✅ Aucune absence non-justifiée !"
	}

	return []*discordgo.MessageEmbed{{
		Title:       "Absences",
		Description: desc,
		Color:       0x3498db,
		Fields:      fields,
	}}, nil
}

// ── /moyenne ─────────────────────────────────────────────────────────────────

func (b *BotCommand) buildMoyenneEmbeds(year uint, dateFilter func(time.Time) bool) ([]*discordgo.MessageEmbed, error) {
	if isTokenExpired() {
		refreshToken(defaultViper.GetString(UsernameViperKey.Key()), credentialsViper.GetString(PasswordViperKey.Key()))
	}
	cfg := buildTokenClientConfiguration()
	grades, err := gaps.NewGradesAction(cfg, year).FetchGrades()
	if err != nil {
		return nil, err
	}

	fields := []*discordgo.MessageEmbedField{}
	for _, class := range grades {
		// If a date filter is active, check that at least one grade passes it
		if dateFilter != nil {
			hasGrade := false
			for _, group := range class.GradeGroups {
				for _, g := range group.Grades {
					if dateFilter(g.Date) {
						hasGrade = true
						break
					}
				}
				if hasGrade {
					break
				}
			}
			if !hasGrade {
				continue
			}
		}

		mean := class.GlobalMean
		if mean == "" || mean == "-" {
			mean = "—"
		}
		emoji := "⚪"
		if v, err2 := strconv.ParseFloat(mean, 32); err2 == nil {
			switch {
			case v >= 5.0:
				emoji = "🟢"
			case v >= 4.0:
				emoji = "🟡"
			default:
				emoji = "🔴"
			}
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   class.Name,
			Value:  fmt.Sprintf("%s **%s**", emoji, mean),
			Inline: true,
		})
	}

	label := fmt.Sprintf("Moyennes %d-%d", year, year+1)
	if len(fields) == 0 {
		return []*discordgo.MessageEmbed{{Title: label, Description: "Aucune note trouvée.", Color: 0x95a5a6}}, nil
	}
	return []*discordgo.MessageEmbed{{Title: label, Color: 0x3498db, Fields: fields}}, nil
}

// ── /recap ────────────────────────────────────────────────────────────────────

func (b *BotCommand) buildRecapEmbeds() ([]*discordgo.MessageEmbed, error) {
	if isTokenExpired() {
		refreshToken(defaultViper.GetString(UsernameViperKey.Key()), credentialsViper.GetString(PasswordViperKey.Key()))
	}
	cfg := buildTokenClientConfiguration()
	grades, err := gaps.NewGradesAction(cfg, currentAcademicYear()).FetchGrades()
	if err != nil {
		return nil, err
	}

	var totalGrades, missingGrades int
	var below4, above5 int
	fields := []*discordgo.MessageEmbedField{}

	for _, class := range grades {
		for _, group := range class.GradeGroups {
			for _, g := range group.Grades {
				totalGrades++
				if g.Grade == "-" || g.Grade == "" {
					missingGrades++
				} else if v, err2 := strconv.ParseFloat(g.Grade, 32); err2 == nil {
					if v < 4.0 {
						below4++
					} else if v >= 5.0 {
						above5++
					}
				}
			}
		}
		mean := class.GlobalMean
		if mean == "" {
			mean = "—"
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   class.Name,
			Value:  fmt.Sprintf("Moyenne : **%s**", mean),
			Inline: true,
		})
	}

	obtained := totalGrades - missingGrades
	desc := fmt.Sprintf(
		"**%d** notes obtenues sur **%d** attendues\n🟢 ≥5 : **%d** | 🟡 4-5 : **%d** | 🔴 <4 : **%d**\n⏳ En attente : **%d**",
		obtained, totalGrades, above5, obtained-above5-below4, below4, missingGrades,
	)

	return []*discordgo.MessageEmbed{{
		Title:       fmt.Sprintf("Récap %d-%d", currentAcademicYear(), currentAcademicYear()+1),
		Description: desc,
		Color:       0x9b59b6,
		Fields:      fields,
	}}, nil
}

// ── /manquantes ───────────────────────────────────────────────────────────────

func (b *BotCommand) buildManquantesEmbeds() ([]*discordgo.MessageEmbed, error) {
	if isTokenExpired() {
		refreshToken(defaultViper.GetString(UsernameViperKey.Key()), credentialsViper.GetString(PasswordViperKey.Key()))
	}
	cfg := buildTokenClientConfiguration()
	grades, err := gaps.NewGradesAction(cfg, currentAcademicYear()).FetchGrades()
	if err != nil {
		return nil, err
	}

	fields := []*discordgo.MessageEmbedField{}
	for _, class := range grades {
		for _, group := range class.GradeGroups {
			for _, g := range group.Grades {
				if g.Grade != "-" && g.Grade != "" {
					continue
				}
				date := "date inconnue"
				if !g.Date.IsZero() {
					date = g.Date.Format("02.01.2006")
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("%s — %s", class.Name, g.Description),
					Value:  fmt.Sprintf("⏳ Prévue le %s | Poids : %.0f%%", date, g.Weight),
					Inline: false,
				})
			}
		}
	}

	if len(fields) == 0 {
		return []*discordgo.MessageEmbed{{
			Title:       "Notes manquantes",
			Description: "✅ Toutes les notes sont publiées !",
			Color:       0x2ecc71,
		}}, nil
	}
	return []*discordgo.MessageEmbed{{
		Title:  fmt.Sprintf("Notes manquantes (%d)", len(fields)),
		Color:  0xe67e22,
		Fields: fields,
	}}, nil
}

// ── /clear ────────────────────────────────────────────────────────────────────

func (b *BotCommand) handleClear(s *discordgo.Session, i *discordgo.InteractionCreate, channelId string) {
	msgs, err := s.ChannelMessages(channelId, 100, "", "", "")
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: "❌ Impossible de récupérer les messages."})
		return
	}

	var ids []string
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}

	if len(ids) == 0 {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: "Le canal est déjà vide."})
		return
	}

	if err := s.ChannelMessagesBulkDelete(channelId, ids); err != nil {
		// Bulk delete fails for messages > 14 days — delete one by one as fallback
		deleted := 0
		for _, id := range ids {
			if s.ChannelMessageDelete(channelId, id) == nil {
				deleted++
			}
		}
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("✅ %d message(s) supprimé(s) (anciens messages : suppression unitaire).", deleted),
		})
		return
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("✅ %d message(s) supprimé(s).", len(ids)),
	})
}
