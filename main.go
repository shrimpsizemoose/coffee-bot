package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pelletier/go-toml/v2"
	"github.com/robfig/cron/v3"
)

type Config struct {
	TelegramToken string   `toml:"telegram_token"`
	ChatID        int64    `toml:"chat_id"`
	Schedule      string   `toml:"schedule"`
	DefaultMsgs   []string `toml:"default_messages"`
	DayMessages   struct {
		Monday    string `toml:"monday"`
		Tuesday   string `toml:"tuesday"`
		Wednesday string `toml:"wednesday"`
		Thursday  string `toml:"thursday"`
		Friday    string `toml:"friday"`
	} `toml:"day_messages"`
	WeekendDays []time.Weekday `toml:"weekend_days"`
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

func getRandomMessage(messages []string) string {
	return messages[rand.Intn(len(messages))]
}

func isWeekend(day time.Weekday, weekendDays []time.Weekday) bool {
	for _, weekend := range weekendDays {
		if day == weekend {
			return true
		}
	}
	return false
}

func getMessages(config *Config, now time.Time) []string {
	if isWeekend(now.Weekday(), config.WeekendDays) {
		log.Printf("No message on weekend at %v", now)
		return nil
	}
	messages := []string{getRandomMessage(config.DefaultMsgs)}

	var customMsg string
	switch now.Weekday() {
	case time.Monday:
		customMsg = config.DayMessages.Monday
	case time.Tuesday:
		customMsg = config.DayMessages.Tuesday
	case time.Wednesday:
		customMsg = config.DayMessages.Wednesday
	case time.Thursday:
		customMsg = config.DayMessages.Thursday
	case time.Friday:
		customMsg = config.DayMessages.Friday
	}

	if customMsg != "" {
		messages = append(messages, customMsg)
	}
	return messages
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, messages []string) error {
	for _, message := range messages {
		msg := tgbotapi.NewMessage(chatID, message)
		if _, err := bot.Send(msg); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	return nil
}

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.toml"
	}
	config, err := loadConfig(configPath)

	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(config.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	c := cron.New()
	_, err = c.AddFunc(config.Schedule, func() {
		now := time.Now()
		messages := getMessages(config, now)
		if len(messages) > 0 {
			if err := sendMessage(bot, config.ChatID, messages); err != nil {
				log.Printf("Failed to send messages: %v", err)
			} else {
				log.Printf("%d Message(s) sent succesfully on %v", len(messages), now)
			}
		}
	})

	if err != nil {
		log.Fatalf("Failed to schedule job: %v", err)
	}

	log.Println("Starting coffee chat notifier...")
	c.Start()

	select {}
}
