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
	debugMode         bool
	skipRetryBackoff  bool // Set to true in tests to skip sleep delays

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

	weatherFetchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coffee_weather_fetch_total",
			Help: "Total number of weather fetch attempts per city",
		},
		[]string{"city", "status"},
	)

	weatherRetryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coffee_weather_retry_total",
			Help: "Total number of weather fetch retries per city",
		},
		[]string{"city"},
	)
)

func init() {
	prometheus.MustRegister(messagesSentTotal)
	prometheus.MustRegister(messageErrorsTotal)
	prometheus.MustRegister(scheduleRunsTotal)
	prometheus.MustRegister(botStartTime)
	prometheus.MustRegister(configuredChats)
	prometheus.MustRegister(weatherFetchTotal)
	prometheus.MustRegister(weatherRetryTotal)
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

func fetchWeatherForCity(city, baseURL string, maxRetries int) (string, error) {
	url := fmt.Sprintf("%s/%s?format=3", baseURL, city)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			// Backoff: 1s, 10s
			var backoff time.Duration
			if attempt == 2 {
				backoff = 1 * time.Second
			} else {
				backoff = 10 * time.Second
			}
			weatherRetryTotal.WithLabelValues(city).Inc()
			if debugMode {
				log.Printf("Retry attempt %d/%d for %s after %v", attempt, maxRetries, city, backoff)
			}
			if !skipRetryBackoff {
				time.Sleep(backoff)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			cancel()
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			cancel()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned status %d", resp.StatusCode)
			resp.Body.Close()
			cancel()
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		line := strings.TrimSpace(string(body))
		if line != "" {
			if attempt > 1 {
				log.Printf("Successfully fetched weather for %s on attempt %d", city, attempt)
			}
			weatherFetchTotal.WithLabelValues(city, "success").Inc()
			return line, nil
		}
		lastErr = fmt.Errorf("empty response")
	}

	weatherFetchTotal.WithLabelValues(city, "failed").Inc()
	return "", lastErr
}

func fetchWeatherFromURL(cities []string, baseURL string) string {
	if len(cities) == 0 {
		return ""
	}

	maxRetries := 3
	log.Printf("Fetching weather for cities: %v (max %d retries per city)", cities, maxRetries)

	var weatherLines []string
	for _, city := range cities {
		line, err := fetchWeatherForCity(city, baseURL, maxRetries)
		if err != nil {
			log.Printf("Failed to fetch weather for %s after %d attempts: %v", city, maxRetries, err)
			continue
		}
		weatherLines = append(weatherLines, line)
	}

	result := strings.Join(weatherLines, "\n")
	if result != "" {
		log.Printf("Weather fetched successfully: %s", result)
	} else {
		log.Printf("No weather data could be fetched for any city")
	}
	return result
}

func fetchWeather(cities []string) string {
	return fetchWeatherFromURL(cities, "http://wttr.in")
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

	for i, message := range messages {
		messageText := message
		// Append weather only to the last message
		if weather != "" && i == len(messages)-1 {
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
