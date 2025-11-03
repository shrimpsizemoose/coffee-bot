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

	t.Run("three cities concatenated with newlines", func(t *testing.T) {
		// Mock server that returns different weather for each city
		callCount := 0
		cityResponses := map[string]string{
			"Barcelona": "Barcelona: ☀️ +20°C",
			"Linkoping": "Linkoping: ☁️ +4°C",
			"Moscow":    "Moscow: 🌨 -2°C",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			// Extract city from path like "/Barcelona?format=3"
			path := strings.TrimPrefix(r.URL.Path, "/")
			city := path

			response, exists := cityResponses[city]
			if !exists {
				t.Errorf("Unexpected city requested: %s", city)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		result := fetchWeatherFromURL([]string{"Barcelona", "Linkoping", "Moscow"}, server.URL)

		// Check that we got 3 separate requests
		if callCount != 3 {
			t.Errorf("Expected 3 HTTP calls, got %d", callCount)
		}

		// Check that result contains all three cities on separate lines
		lines := strings.Split(result, "\n")
		if len(lines) != 3 {
			t.Errorf("Expected 3 lines in result, got %d: %v", len(lines), lines)
		}

		if !strings.Contains(result, "Barcelona: ☀️ +20°C") {
			t.Errorf("Result missing Barcelona weather: %s", result)
		}
		if !strings.Contains(result, "Linkoping: ☁️ +4°C") {
			t.Errorf("Result missing Linkoping weather: %s", result)
		}
		if !strings.Contains(result, "Moscow: 🌨 -2°C") {
			t.Errorf("Result missing Moscow weather: %s", result)
		}

		t.Logf("✓ Weather correctly concatenated:\n%s", result)
	})

	t.Run("single city", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("London: ⛅ +12°C"))
		}))
		defer server.Close()

		result := fetchWeatherFromURL([]string{"London"}, server.URL)
		if result != "London: ⛅ +12°C" {
			t.Errorf("Expected 'London: ⛅ +12°C', got '%s'", result)
		}
	})
}
