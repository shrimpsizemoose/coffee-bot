# Coffee Bot Notifier

A Telegram bot that sends scheduled coffee/tea reminder messages to multiple chats with support for topics.

## Features

- **Multi-chat support**: Send messages to multiple Telegram chats simultaneously
- **Topic support**: Send messages to specific topics in supergroups
- **Custom schedules**: Configurable cron schedules for message delivery
- **Day-specific messages**: Different messages for different days of the week
- **Weekend handling**: Skip messages on configured weekend days
- **Weather integration**: Optional per-chat weather reports from wttr.in
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

# Chat with topic support and weather
[[chats]]
chat_id = -1009876543210
topic_id = 2
alias = "dev_team"
weather_enabled = true
weather_cities = ["Katmandu", "Perth"]
default_messages = ["Morning team! ☕"]
day_messages = { friday = "TGIF coffee break!" }
```

## Weather Integration

Add optional weather reports to your messages on a per-chat basis:

```toml
[[chats]]
chat_id = -1001234567890
alias = "morning_chat"
weather_enabled = true
weather_cities = ["Katmandu", "Perth"]
default_messages = ["Good morning! ☕"]
```

**Features:**
- Enable/disable weather per chat with `weather_enabled`
- Specify multiple cities with `weather_cities` array
- Weather data fetched from wttr.in API
- Weather information appended after your message text
- Automatic timeout handling (10 seconds per attempt)
- **Automatic retry with backoff**: Up to 3 attempts per city (1s, 10s backoff)
- Failed cities are logged but don't block successful fetches

**Example output:**
```
Good morning! ☕

Katmandu: ⛅️ +18°C
Perth: ☀️ +25°C
```

## Prometheus Metrics

The bot exposes metrics on `http://localhost:8080/metrics` for monitoring:

- `coffee_messages_sent_total{chat_alias, has_topic}` - Count of sent messages
- `coffee_message_errors_total{chat_alias, error_type}` - Count of send errors
- `coffee_schedule_runs_total` - Count of cron job executions
- `coffee_bot_start_time_seconds` - Bot startup timestamp
- `coffee_configured_chats_total` - Number of configured chats
- `coffee_weather_fetch_total{city, status}` - Weather fetch attempts per city (success/failed)
- `coffee_weather_retry_total{city}` - Weather fetch retries per city

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

### Release Pipeline

```
Development → Push → build-and-test ✅
           ↓
    Ready for release → Manual release → All checks again ✅
                                     ↓
                                Tag created ✅
                                     ↓
                               docker-build → Docker image ✅
```

**Workflows:**
- `build-and-test.yml` - Runs on every push/PR (tests, build, Go version check)
- `release.yml` - Manual workflow to create tags after full validation
- `docker-build.yml` - Automatically builds and publishes Docker images when tags are created

**To create a release:**
1. Go to Actions → Release → Run workflow
2. Enter version (e.g., `v1.2`)
3. Workflow validates, tests, and creates tag
4. Docker image automatically publishes to `ghcr.io/shrimpsizemoose/coffee-bot:v1.2`
