package main

import (
	"testing"
	"github.com/pelletier/go-toml/v2"
)


func TestConfigParsing(t *testing.T) {
	// Тест парсинга нового формата конфига
	data := []byte(`
telegram_token = "123456789:FAKE_TOKEN"
schedule = "*/1 * * * *"
weekend_days = []

[[chats]]
chat_id = -1001111111111
default_messages = ["Тест 1", "Тест 2"]
day_messages = { monday = "Понедельник", tuesday = "Вторник" }

[[chats]]
chat_id = -1002222222222
topic_id = 2
default_messages = ["Топик тест"]
day_messages = { friday = "Пятница в топике" }
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

	// Второй чат
	chat2 := config.Chats[1]
	if chat2.TopicID == nil || *chat2.TopicID != 2 {
		t.Errorf("Expected topic ID 2 for second chat")
	}

	t.Logf("✓ Config parsed successfully with %d chats", len(config.Chats))
}