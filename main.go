package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"time"

	"github.com/mymmrac/telego"
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

func (c *Config) Tell() {
	if len(c.TelegramToken) > 0 {
		log.Println("Bot token is set 👍🏻")
	}
	log.Printf("Running on cron schedule '%s' in chat %d", c.Schedule, c.ChatID)
	log.Printf("I have %d predefined default message (triggered every day)", len(c.DefaultMsgs))

	var customs []string
	v := reflect.ValueOf(c.DayMessages)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Len() > 0 {
			day := v.Type().Field(i).Name
			customs = append(customs, day)
		}
	}
	log.Printf("Custom messages are set for: %v", customs)
	log.Printf("Silent days: %v", c.WeekendDays)
}

func (c *Config) getRandomMessage() string {
	return c.DefaultMsgs[rand.Intn(len(c.DefaultMsgs))]
}

func (c *Config) isWeekend(day time.Weekday) bool {
	for _, weekend := range c.WeekendDays {
		if day == weekend {
			return true
		}
	}
	return false
}

func (c *Config) PickMessages(now time.Time) []string {
	if c.isWeekend(now.Weekday()) {
		log.Printf("No message on weekend at %v", now)
		return nil
	}
	messages := []string{c.getRandomMessage()}

	var customMsg string
	switch now.Weekday() {
	case time.Monday:
		customMsg = c.DayMessages.Monday
	case time.Tuesday:
		customMsg = c.DayMessages.Tuesday
	case time.Wednesday:
		customMsg = c.DayMessages.Wednesday
	case time.Thursday:
		customMsg = c.DayMessages.Thursday
	case time.Friday:
		customMsg = c.DayMessages.Friday
	}

	if customMsg != "" {
		messages = append(messages, customMsg)
	}
	return messages
}

func sendMessage(bot *telego.Bot, chatID int64, messages []string) error {
	for _, message := range messages {
		params := &telego.SendMessageParams{
			ChatID: telego.ChatID{ID: chatID},
			Text:   message,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := bot.SendMessage(ctx, params)
		if err != nil {
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
	config.Tell()

	bot, err := telego.NewBot(config.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	c := cron.New()
	_, err = c.AddFunc(config.Schedule, func() {
		now := time.Now()
		messages := config.PickMessages(now)
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
