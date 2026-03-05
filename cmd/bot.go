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
			Name:        "absences",
			Description: "Affiche tes absences",
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

		// Acknowledge immediately (Discord requires response within 3s)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		var embeds []*discordgo.MessageEmbed
		var err error

		data := i.ApplicationCommandData()
		switch data.Name {
		case "notes":
			year := currentAcademicYear()
			for _, opt := range data.Options {
				if opt.Name == "semestre" {
					year = semesterToYear(int(opt.IntValue()))
				}
			}
			embeds, err = b.buildGradesEmbeds(year)
		case "allnotes":
			embeds, err = b.buildAllGradesEmbeds()
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
		e, err := b.buildGradesEmbeds(year)
		if err != nil {
			return nil, err
		}
		embeds = append(embeds, e...)
	}
	return embeds, nil
}

func (b *BotCommand) buildGradesEmbeds(year uint) ([]*discordgo.MessageEmbed, error) {
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
			for _, g := range group.Grades {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   g.Description,
					Value:  fmt.Sprintf("Note: **%s** | Moy: **%s** | Poids: %.0f%%", g.Grade, g.ClassMean, g.Weight),
					Inline: false,
				})
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("— Moyenne %s", group.Name),
				Value:  fmt.Sprintf("**%s** (poids %d%%)", group.Mean, group.Weight),
				Inline: false,
			})
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
