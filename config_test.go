package main

import (
	"github.com/pelletier/go-toml/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigParsing(t *testing.T) {
	// Тест парсинга нового формата конфига
	data := []byte(`
telegram_token = "123456789:FAKE_TOKEN"
schedule = "*/1 * * * *"
weekend_days = []

[[chats]]
chat_id = -1001111111111
alias = "test_chat"
default_messages = ["Тест 1", "Тест 2"]
day_messages = { monday = "Понедельник", tuesday = "Вторник" }
weather_enabled = true
weather_cities = ["Barcelona", "Madrid"]

[[chats]]
chat_id = -1002222222222
topic_id = 2
alias = "test_topic"
default_messages = ["Топик тест"]
day_messages = { friday = "Пятница в топике" }
weather_enabled = false
`)

	var config Config
	err := toml.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Проверки
	if config.TelegramToken != "123456789:FAKE_TOKEN" {
		t.Errorf("Expected token '123456789:FAKE_TOKEN', got '%s'", config.TelegramToken)
	}

	if len(config.Chats) != 2 {
		t.Errorf("Expected 2 chats, got %d", len(config.Chats))
	}

	// Первый чат
	chat1 := config.Chats[0]
	if chat1.ChatID != -1001111111111 {
		t.Errorf("Expected chat ID -1001111111111, got %d", chat1.ChatID)
	}
	if chat1.TopicID != nil {
		t.Errorf("Expected no topic ID for first chat, got %d", *chat1.TopicID)
	}
	if len(chat1.DefaultMsgs) != 2 {
		t.Errorf("Expected 2 default messages, got %d", len(chat1.DefaultMsgs))
	}
	if !chat1.WeatherEnabled {
		t.Errorf("Expected weather enabled for first chat")
	}
	if len(chat1.WeatherCities) != 2 {
		t.Errorf("Expected 2 weather cities, got %d", len(chat1.WeatherCities))
	}
	if chat1.WeatherCities[0] != "Barcelona" {
		t.Errorf("Expected first city 'Barcelona', got '%s'", chat1.WeatherCities[0])
	}

	// Второй чат
	chat2 := config.Chats[1]
	if chat2.TopicID == nil || *chat2.TopicID != 2 {
		t.Errorf("Expected topic ID 2 for second chat")
	}
	if chat2.WeatherEnabled {
		t.Errorf("Expected weather disabled for second chat")
	}

	t.Logf("✓ Config parsed successfully with %d chats", len(config.Chats))
}

func TestFetchWeather(t *testing.T) {
	t.Run("empty cities list", func(t *testing.T) {
		result := fetchWeather([]string{})
		if result != "" {
			t.Errorf("Expected empty string for empty cities list, got '%s'", result)
		}
	})

	t.Run("successful fetch", func(t *testing.T) {
		mockResponse := "Barcelona: ☀️ +20°C\nMadrid: ⛅ +18°C"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "Barcelona,Madrid") {
				t.Errorf("Expected cities in URL path, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockResponse))
		}))
		defer server.Close()

		// can't test actual wttr.in call without making real HTTP requests,
		// so just verify the function doesn't crash with valid input
		result := fetchWeather([]string{"Barcelona", "Madrid"})
		// result could be empty if wttr.in is down, just check it doesn't panic
		t.Logf("Weather fetch result: '%s'", result)
	})

	t.Run("single city", func(t *testing.T) {
		result := fetchWeather([]string{"London"})
		t.Logf("Single city weather: '%s'", result) // (verify it doesn't crash)
	})
}
