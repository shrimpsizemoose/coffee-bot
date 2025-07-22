# Coffee Bot Notifier

A Telegram bot that sends scheduled coffee/tea reminder messages to multiple chats with support for topics.

## Features

- **Multi-chat support**: Send messages to multiple Telegram chats simultaneously
- **Topic support**: Send messages to specific topics in supergroups
- **Custom schedules**: Configurable cron schedules for message delivery
- **Day-specific messages**: Different messages for different days of the week
- **Weekend handling**: Skip messages on configured weekend days
- **Flexible configuration**: TOML-based configuration with inline tables

## Quick Start

```bash
cp config.toml.template config.toml
vim config.toml  # Configure your bot token and chats
docker compose up -d
```

## Configuration Format

The bot uses a multi-chat configuration format:

```toml
telegram_token = "YOUR_BOT_TOKEN"
schedule = "5 7 * * *"  # 7:05 AM daily
weekend_days = [6, 0]   # Saturday, Sunday as "silent days"

# Regular chat
[[chats]]
chat_id = -1001234567890
default_messages = ["Good morning! Coffee time?"]
day_messages = { monday = "Monday motivation!" }

# Chat with topic support  
[[chats]]
chat_id = -1009876543210
topic_id = 2
default_messages = ["Morning team! ☕"]
day_messages = { friday = "TGIF coffee break!" }
```

## Development

```bash
# Run tests
go test -v

# Build
go build -o coffee-bot main.go

# Run with custom config
CONFIG_PATH=config-test.toml ./coffee-bot
```
