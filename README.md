# Flashnet Market Monitor

Go-based Telegram bot for monitoring Flashnet/Spark AMM activity with real-time notifications, chart generation, and comprehensive market analytics.

## Features

- **Big Sales Monitor**: Track large swaps (buys/sells) above configurable BTC thresholds
- **Hot Token Detection**: Identify tokens with high swap activity and multiple unique addresses
- **Holder Dynamics**: Monitor token holder changes, investments, and liquidations
- **Statistics & Reports**: Daily volume statistics, holder reports, and flow analysis
- **Chart Generation**: Automatic generation of volume charts and BTC spark charts
- **Filtered Token Monitoring**: Monitor specific tokens with custom thresholds
- **Real-time Telegram Notifications**: Instant alerts for significant market events

## Requirements

- Go 1.24.0 or higher
- Node.js (for challenge signing)
- Telegram Bot Token
- Flashnet API access (public key and wallet)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/Jas952/flashnet-market-monitor.git
cd flashnet-market-monitor
```

2. Install Go dependencies:
```bash
go mod download
```

3. Install Node.js dependencies:
```bash
cd spark-cli
npm install
cd ..
```

4. Configure the bot:
   - Copy `.env.example` to `.env` and fill in your secrets
   - Copy `config.yaml.example` to `config.yaml` and adjust settings

## Configuration

### Environment Variables (.env)

Create `.env` file with the following variables:

```env
# Flashnet API Configuration
PUBLIC_KEY=your_wallet_public_key

# Telegram Bot Tokens
TELEGRAM_BOT1_TOKEN=your_bot_token
TELEGRAM_BOT2_TOKEN=your_second_bot_token
API_BOT_TOKEN=your_api_bot_token

# Telegram Chat IDs
BIG_SALES_CHAT_ID=your_chat_id
API_BOT_CHAT_ID=your_chat_id
FILTERED_CHAT_ID=your_chat_id
```

### Configuration File (config.yaml)

Create `config.yaml` based on `config.yaml.example`:

```yaml
monitoring:
  big_sales_min_btc_amount: 0.0025
  filtered_min_btc_amount: 0.01
  stats_send_time: "10:00"
  hot_token:
    swaps_count: 6
    min_addresses: 3

telegram:
  filtered_tokens:
    - "token_pool_lp_public_key_1"
    - "token_pool_lp_public_key_2"

app:
  check_interval: 60

flashnet:
  network: "mainnet"
  request_timeout: 30
  max_retries: 3
```

## Usage

### Authentication

First, authenticate with Flashnet API:

```bash
make auth-token
```

This will:
1. Get challenge from API
2. Sign it using spark-cli
3. Verify and save JWT token

### Running the Bot

**Full bot with Telegram notifications:**
```bash
make run-bot
```

**Big Sales monitor only (no Telegram):**
```bash
make run-big-sales
```

**Holders dynamics monitor:**
```bash
make run-holders
```

### Available Make Commands

```bash
make help              # Show all available commands
make auth-token        # Complete authentication flow
make sign              # Sign challenge only
make run-bot           # Run full bot
make run-big-sales     # Run big sales monitor
make run-holders       # Run holders monitor
make clean             # Remove build artifacts
```

## Project Structure

```
.
├── cmd/                    # Application entry points
│   ├── auth/              # Authentication utilities
│   ├── big_sales/         # Big sales monitor
│   ├── bot/               # Main bot application
│   └── holders/           # Holders monitor
├── http_client/
│   ├── bot/               # Telegram bot modules
│   │   ├── big_sales_monitor.go
│   │   ├── hot_token_monitor.go
│   │   ├── holders_dynamic_monitor.go
│   │   └── stats_monitor.go
│   └── system_works/      # Core system modules
│       ├── http_client.go  # Flashnet API client
│       ├── config.go      # Configuration management
│       ├── logger.go      # Logging utilities
│       └── ...
├── spark-cli/             # Challenge signing (Node.js)
├── etc/                   # Assets and tools
│   ├── charts/            # Generated charts
│   ├── telegram/          # Telegram assets
│   └── tools/             # Utility scripts
├── data_in/               # Input data (challenges, tokens)
├── data_out/              # Output data (logs, reports)
├── config.yaml.example    # Configuration template
├── .env.example           # Environment variables template
└── Makefile               # Build and run commands
```

## Modules

### Big Sales Monitor
Monitors AMM swaps and notifies about large transactions exceeding configured BTC thresholds.

### Hot Token Monitor
Detects tokens with high activity based on:
- Number of swaps in time window
- Number of unique addresses

### Holders Dynamic Monitor
Tracks token holder changes:
- New investments
- Sales
- Liquidations
- Daily holder counts

### Statistics Monitor
Generates and sends daily statistics:
- Volume charts
- BTC spark charts
- Holder reports
- Flow analysis

## Data Storage

- `data_in/`: Authentication data (challenges, signatures, tokens)
- `data_out/`: Runtime data
  - `big_sales_module/`: Big sales tracking data
  - `holders_module/`: Holders dynamics data
  - `telegram_out/`: Generated reports and statistics

## API Integration

- **Flashnet API**: Main AMM swap data and authentication
- **Luminex API**: Token metadata, holder information, wallet balances

## Development

### Building

```bash
go build ./cmd/bot/main.go
```

### Testing

```bash
go test ./...
```

### Code Quality

The project uses:
- `golangci-lint` for code analysis
- `.editorconfig` for consistent formatting

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]

## Support

[Add support information here]


