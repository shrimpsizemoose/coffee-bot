package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/pelletier/go-toml/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
)

type ChatConfig struct {
	ChatID         int64             `toml:"chat_id"`
	TopicID        *int              `toml:"topic_id,omitempty"`
	Alias          string            `toml:"alias"`
	DefaultMsgs    []string          `toml:"default_messages"`
	DayMessages    map[string]string `toml:"day_messages"`
	WeatherEnabled bool              `toml:"weather_enabled"`
	WeatherCities  []string          `toml:"weather_cities"`
}

type Config struct {
	TelegramToken string         `toml:"telegram_token"`
	Schedule      string         `toml:"schedule"`
	WeekendDays   []time.Weekday `toml:"weekend_days"`
	Chats         []ChatConfig   `toml:"chats"`
}

var (
	debugMode bool

	messagesSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coffee_messages_sent_total",
			Help: "Total number of coffee messages sent",
		},
		[]string{"chat_alias", "has_topic"},
	)

	messageErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coffee_message_errors_total",
			Help: "Total number of message send errors",
		},
		[]string{"chat_alias", "error_type"},
	)

	scheduleRunsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "coffee_schedule_runs_total",
			Help: "Total number of scheduled job runs",
		},
	)

	botStartTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "coffee_bot_start_time_seconds",
			Help: "Unix timestamp when the bot was started",
		},
	)

	configuredChats = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "coffee_configured_chats_total",
			Help: "Number of configured chats",
		},
	)
)

func init() {
	prometheus.MustRegister(messagesSentTotal)
	prometheus.MustRegister(messageErrorsTotal)
	prometheus.MustRegister(scheduleRunsTotal)
	prometheus.MustRegister(botStartTime)
	prometheus.MustRegister(configuredChats)
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	return &config, nil
}

func (c *Config) Tell() {
	if len(c.TelegramToken) > 0 {
		log.Println("Bot token is set 👍🏻")
	}
	log.Printf("Running on schedule '%s' for %d chat(s)", c.Schedule, len(c.Chats))
	log.Printf("Silent days: %v", c.WeekendDays)

	for i, chat := range c.Chats {
		topicInfo := ""
		if chat.TopicID != nil {
			topicInfo = fmt.Sprintf(" (topic %d)", *chat.TopicID)
		}
		log.Printf("Chat %d: ID=%d%s, %d default messages", i+1, chat.ChatID, topicInfo, len(chat.DefaultMsgs))

		var customDays []string
		for day, msg := range chat.DayMessages {
			if msg != "" {
				customDays = append(customDays, day)
			}
		}
		if len(customDays) > 0 {
			log.Printf("  Custom messages for: %v", customDays)
		}

		if chat.WeatherEnabled {
			log.Printf("  Weather enabled for cities: %v", chat.WeatherCities)
		}
	}
}

func fetchWeather(cities []string) string {
	if len(cities) == 0 {
		return ""
	}

	log.Printf("Fetching weather for cities: %v", cities)

	citiesParam := strings.Join(cities, ",")
	url := fmt.Sprintf("http://wttr.in/{%s}?format=3", citiesParam)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("Failed to create weather request: %v", err)
		return ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch weather: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Weather API returned status %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read weather response: %v", err)
		return ""
	}

	result := strings.TrimSpace(string(body))
	log.Printf("Weather fetched successfully: %s", result)
	return result
}

func (cc *ChatConfig) getRandomMessage() string {
	return cc.DefaultMsgs[rand.Intn(len(cc.DefaultMsgs))]
}

func (cc *ChatConfig) PickMessages(now time.Time, weekendDays []time.Weekday) []string {
	/*
		Message selection logic:
		- Both default and day-specific exist: send random default + day-specific message
		- Only day-specific exists (no defaults): send only the day-specific message
		- Only defaults exist (no day-specific for today): send random default message
		- Neither exist: send nothing
	*/

	for _, weekend := range weekendDays {
		if now.Weekday() == weekend {
			log.Printf("No message on weekend at %v", now)
			return nil
		}
	}

	dayName := map[time.Weekday]string{
		time.Monday:    "monday",
		time.Tuesday:   "tuesday",
		time.Wednesday: "wednesday",
		time.Thursday:  "thursday",
		time.Friday:    "friday",
	}

	var messages []string
	var hasDayMessage bool

	if dayKey, exists := dayName[now.Weekday()]; exists {
		if customMsg, hasMsg := cc.DayMessages[dayKey]; hasMsg && customMsg != "" {
			hasDayMessage = true
			if len(cc.DefaultMsgs) > 0 {
				messages = append(messages, cc.getRandomMessage())
			}
			messages = append(messages, customMsg)
		}
	}

	if !hasDayMessage && len(cc.DefaultMsgs) > 0 {
		messages = append(messages, cc.getRandomMessage())
	}

	if len(messages) == 0 && debugMode {
		log.Printf("No messages available to send (chat has no default messages and no day-specific message for today)")
	}

	return messages
}

func sendMessage(bot *telego.Bot, chatConfig ChatConfig, messages []string) error {
	chatAlias := chatConfig.Alias
	if chatAlias == "" {
		chatAlias = fmt.Sprintf("chat_%d", chatConfig.ChatID)
	}
	hasTopic := "false"
	if chatConfig.TopicID != nil {
		hasTopic = "true"
	}

	var weather string
	if chatConfig.WeatherEnabled {
		log.Printf("Weather enabled for chat %s, fetching...", chatAlias)
		weather = fetchWeather(chatConfig.WeatherCities)
		if weather == "" {
			log.Printf("Weather fetch returned empty for chat %s", chatAlias)
		}
	}

	for _, message := range messages {
		messageText := message
		if weather != "" {
			messageText = fmt.Sprintf("%s\n\n%s", message, weather)
		}

		params := &telego.SendMessageParams{
			ChatID: telego.ChatID{ID: chatConfig.ChatID},
			Text:   messageText,
		}

		if chatConfig.TopicID != nil {
			params.MessageThreadID = *chatConfig.TopicID
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := bot.SendMessage(ctx, params)
		if err != nil {
			messageErrorsTotal.WithLabelValues(chatAlias, "send_failed").Inc()
			return fmt.Errorf("failed to send message to chat %d: %w", chatConfig.ChatID, err)
		}

		messagesSentTotal.WithLabelValues(chatAlias, hasTopic).Inc()
	}

	return nil
}

func main() {
	debugMode = os.Getenv("DEBUG") == "true"
	if debugMode {
		log.Println("Debug mode enabled")
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.toml"
	}
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	config.Tell()

	botStartTime.Set(float64(time.Now().Unix()))
	configuredChats.Set(float64(len(config.Chats)))

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Starting metrics server on :8080/metrics")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		}
	}()

	bot, err := telego.NewBot(config.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}
	log.Println("Telegram bot initialized successfully")

	c := cron.New()
	_, err = c.AddFunc(config.Schedule, func() {
		scheduleRunsTotal.Inc()
		now := time.Now()

		for _, chatConfig := range config.Chats {
			messages := chatConfig.PickMessages(now, config.WeekendDays)
			if len(messages) == 0 {
				if debugMode {
					log.Printf("No messages selected for chat %d on %v", chatConfig.ChatID, now)
				}
				continue
			}
			if err := sendMessage(bot, chatConfig, messages); err != nil {
				log.Printf("Failed to send messages to chat %d: %v", chatConfig.ChatID, err)
			} else {
				topicInfo := ""
				if chatConfig.TopicID != nil {
					topicInfo = fmt.Sprintf(" (topic %d)", *chatConfig.TopicID)
				}
				log.Printf("%d Message(s) sent to chat %d%s on %v", len(messages), chatConfig.ChatID, topicInfo, now)
			}
		}
	})

	if err != nil {
		log.Fatalf("Failed to schedule job: %v", err)
	}
	log.Printf("Cron job scheduled successfully with pattern: %s", config.Schedule)

	log.Println("Starting coffee chat notifier...")
	c.Start()

	select {}
}
