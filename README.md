# Coffee Bot Notifier

A Telegram bot that sends scheduled coffee/tea reminder messages to multiple chats with support for topics.

## Features

- **Multi-chat support**: Send messages to multiple Telegram chats simultaneously
- **Topic support**: Send messages to specific topics in supergroups
- **Custom schedules**: Configurable cron schedules for message delivery
- **Day-specific messages**: Different messages for different days of the week
- **Weekend handling**: Skip messages on configured weekend days
- **Prometheus metrics**: Built-in monitoring and observability
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
alias = "main_chat"
default_messages = ["Good morning! Coffee time?"]
day_messages = { monday = "Monday motivation!" }

# Chat with topic support  
[[chats]]
chat_id = -1009876543210
topic_id = 2
alias = "dev_team"
default_messages = ["Morning team! ☕"]
day_messages = { friday = "TGIF coffee break!" }
```

## Prometheus Metrics

The bot exposes metrics on `http://localhost:8080/metrics` for monitoring:

- `coffee_messages_sent_total{chat_alias, has_topic}` - Count of sent messages
- `coffee_message_errors_total{chat_alias, error_type}` - Count of send errors
- `coffee_schedule_runs_total` - Count of cron job executions
- `coffee_bot_start_time_seconds` - Bot startup timestamp
- `coffee_configured_chats_total` - Number of configured chats

**Privacy**: Metrics use the `alias` field from your config, never real chat IDs.

### Prometheus Configuration

Add to your Prometheus config:

```yaml
- job_name: 'coffee-bot'
  static_configs:
    - targets: ['your-host:8080']
  scrape_interval: 15s
```

## Development

```bash
# Run tests
go test -v

# Build
go build -o coffee-bot main.go

# Run with custom config
CONFIG_PATH=config-test.toml ./coffee-bot

# Check metrics
curl http://localhost:8080/metrics
```
