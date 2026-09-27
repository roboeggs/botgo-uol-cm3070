# SR Levels — Crypto Support/Resistance Bot

Telegram bot for automated support and resistance level analysis on
cryptocurrency markets. Scans Binance pairs daily, detects active
levels, predicts breakout probability using a machine learning model,
and provides AI-powered market analysis.

**CM3070 Computer Science Final Project — University of London.**

---

## Try it online

The bot is deployed and can be tested without any local setup:

**Telegram bot:** https://t.me/SRLevelsUoLBot

If the bot is not responding, it is likely asleep on the free hosting
tier. Wake it up by opening the following link in a browser (a blank
page will load — this is expected, just wait ~30 seconds):

**Wake-up link:** https://botgo-a8s9.onrender.com/

Once awake, return to the bot and send `/start`.

---

## Features

- **Daily level scanning** — active support and resistance levels for
  Binance pairs with sufficient liquidity
- **ML breakout prediction** — probability of level breakout at
  1h, 4h, and 24h horizons, based on a model trained on 300+ coins
- **Touch statistics** — number of touches, first/last touch
  timestamps, level age
- **AI analysis** — natural language market commentary via OpenAI
  (GPT-6 Luna)
- **Multi-language UI** — Russian and English
- **Web App integration** — interactive level browser and market
  screener via Telegram Web Apps
- **Favorites** — per-user saved ticker list

## Bot commands

| Command | Description |
|---|---|
| `/start` | Initialize the bot and show the main menu |
| `/help` | Show usage help |
| `/db <TICKER>` | Fetch a ticker card (e.g. `/db BTCUSDT`) |
| `/db_tickers` | List all tickers with active levels |
| `/ai <question>` | Ask the AI assistant (e.g. `/ai what's interesting?`) |

The bot also accepts plain ticker input — typing `BTC` or `SUIUSDT`
returns the ticker card directly.

---

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.27 |
| Bot framework | `gopkg.in/telebot.v4` |
| Database | PostgreSQL 17+ |
| DB driver | `github.com/jackc/pgx/v5` |
| LLM | OpenAI GPT-6 Luna |
| Market data | Binance public REST API |
| Localisation | `github.com/nicksnyder/go-i18n/v2` |
| Testing | `github.com/stretchr/testify`, `github.com/DATA-DOG/go-sqlmock` |
| Web App | Vanilla JavaScript, TradingView widgets |
| Deployment | Render (backend), GitHub Pages (static Web App) |

---

## Running locally

### Prerequisites

- Go 1.27 or newer
- PostgreSQL 14 or newer
- A Telegram bot token from [@BotFather](https://t.me/BotFather)
- An OpenAI API key (only required for `/ai` functionality)

### 1. Clone the repository

```bash
git clone <repository-url>
cd botgo
```

### 2. Configure the environment

Copy `env_example` to `.env` and fill in the values:

```env
BOT_TOKEN=telegram_bot_token
ADMIN_ID=telegram_user_id
DATABASE_URL=postgres://user:password@localhost:5432/postgres?sslmode=require&default_query_exec_mode=simple_protocol
LOG_LEVEL=debug
WEBAPP_URL=http://localhost:10000
OPENAI_API_KEY=sk-...
```

| Variable | Required | Description |
|---|---|---|
| `BOT_TOKEN` | yes | Telegram bot token from @BotFather |
| `ADMIN_ID` | yes | Telegram user ID of the administrator |
| `DATABASE_URL` | yes | PostgreSQL connection string |
| `LOG_LEVEL` | no | `debug`, `info`, `warn`, `error` (default: `info`) |
| `WEBAPP_URL` | no | Base URL for Web App pages |
| `OPENAI_API_KEY` | for `/ai` | OpenAI API key |

For production, `WEBAPP_URL` should point to the deployed static host,
for example:

```env
WEBAPP_URL=https://roboeggs.github.io/
```

### 3. Set up the database

Ensure the PostgreSQL server is running and the database referenced in
`DATABASE_URL` exists. The required tables are created automatically on
first run.

### 4. Install dependencies

```bash
go mod download
```

### 5. Run the bot

```bash
go run ./cmd/bot/main.go
```

On success, the log will contain:

```
The bot has been successfully launched...
Health server listening port=10000
```

The bot is now reachable in Telegram.

### 6. Run tests

```bash
go test ./...
```

To include the live Binance integration test and the OpenAI integration
tests:

```bash
OPENAI_API_KEY=your_key go test ./... -v
```

Tests that require an external API key are skipped automatically when
the corresponding variable is not set.

---

## Project structure

```
botgo/
├── cmd/bot/              application entry point
├── internal/
│   ├── config/           environment loading
│   ├── database/         PostgreSQL layer (users, tickers, favorites)
│   ├── generate/         AI module (classifier, analyser, tool executor)
│   ├── handlers/         Telegram handlers and middleware
│   ├── i18n/             localisation (RU / EN)
│   ├── market/           Binance client and indicators (EMA, RSI)
│   └── telegram/         message splitter and outbound sender
├── static/               Web App pages (app.html, chart.html)
├── env_example           environment template
├── go.mod
└── README.md
```
---

## Database schema

PostgreSQL 17 is used as the primary datastore. The application relies
on four tables; the full DDL is available in
[`docs/schema.sql`](docs/schema.sql).

### `users`

Registered Telegram users.

| Column | Type | Notes |
|---|---|---|
| `id` | `bigint` | Primary key. Telegram user ID |
| `username` | `text` | Optional, from Telegram |
| `first_name` | `text` | Optional, from Telegram |
| `last_name` | `text` | Optional, from Telegram |
| `language_code` | `text` | UI language, default `'en'` |
| `created_at` | `timestamptz` | Default `now()` |
| `updated_at` | `timestamptz` | Auto-updated by trigger |
| `is_blocked` | `boolean` | Reserved for future moderation |
| `metadata` | `jsonb` | Reserved for extended attributes |

### `user_favorites`

Per-user list of saved tickers.

| Column | Type | Notes |
|---|---|---|
| `user_id` | `bigint` | FK → `users(id)` with `ON DELETE CASCADE` |
| `ticker` | `text` | Uppercased Binance symbol |
| `added_at` | `timestamptz` | Default `now()` |

**Constraints:**

- Composite primary key `(user_id, ticker)` prevents duplicates.
- `CHECK (ticker ~ '^[A-Z0-9]{1,12}USDT$')` enforces a strict ticker
  format at the database level, defending against malformed or
  oversized input.
- Index on `user_id` for fast per-user lookups.

### `detector_state`

Maintained by the external ML pipeline. One row per Binance pair.

| Column | Type | Notes |
|---|---|---|
| `ticker` | `text` | Primary key |
| `state_blob` | `jsonb` | Full detector state with confirmed levels and per-level ML predictions |
| `has_active_levels` | `boolean` | Denormalised flag for fast filtering |
| `saved_at` | `timestamptz` | Last write timestamp |

### `processing_log`

Tracks the last processing time per ticker, used to display data
freshness in ticker cards.

| Column | Type | Notes |
|---|---|---|
| `ticker` | `text` | Primary key |
| `last_processed_time` | `bigint` | Unix timestamp (seconds) |
| `first_loaded_time` | `bigint` | Unix timestamp (seconds) |
| `params_json` | `jsonb` | Scanner configuration snapshot |
| `created_at` | `timestamptz` | Default `now()` |
| `updated_at` | `timestamptz` | Auto-updated by trigger |

### Design notes

- **Triggers** `update_users_updated_at` and
  `trg_processing_log_updated_at` keep the `updated_at` column in sync
  on every update.
- **Denormalised flag** `has_active_levels` allows the bot to list
  active tickers with a single indexed lookup, avoiding a full scan of
  the JSON blob.
- **Tick format check** on `user_favorites` is the last line of defence:
  even if the Go layer fails to validate input, the database rejects
  malformed tickers.
---

## How the ML prediction works

A separate offline pipeline scans Binance pairs and produces a state
blob for each ticker. The blob contains confirmed support and resistance
levels together with a set of per-level features (touch count, touch
density, ATR, volume ratios, BTC correlation, and others).

A gradient boosting model, trained on more than 300 coins, estimates
the probability that each level will be broken within 1, 4, and 24
hours. These probabilities are stored alongside the level and exposed
to the bot and the Web App.

The model itself is maintained as a separate backend service and is not
part of this repository. Predictions are written into the
`detector_state` table in PostgreSQL.

---

## Key dependencies

| Package | Purpose |
|---|---|
| `gopkg.in/telebot.v4` | Telegram Bot API |
| `github.com/openai/openai-go` | OpenAI API client |
| `github.com/jackc/pgx/v5` | PostgreSQL driver |
| `github.com/nicksnyder/go-i18n/v2` | Localisation |
| `github.com/joho/godotenv` | `.env` file loader |
| `gopkg.in/yaml.v3` | YAML parsing for locale files |
| `github.com/stretchr/testify` | Test assertions |
| `github.com/DATA-DOG/go-sqlmock` | SQL mock for database tests |

---

## Disclaimer

This bot is an academic project. It is not financial advice. Trading
cryptocurrencies carries significant risk.

## License

For academic use only.
