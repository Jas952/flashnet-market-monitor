# Flashnet Market Monitor

Go-based Telegram bot for monitoring Flashnet/Spark AMM activity with real-time notifications, chart generation, and comprehensive market analytics.

## About Flashnet

**Flashnet** is a modular exchange stack for Bitcoin. It is designed to rival the performance of TradFi exchange systems without any of the custody, and without introducing blockchain-related inefficiencies to execution. This is facilitated by **Spark**, a UTXO scaling solution that enables near-instant, zero-fee settlement of Bitcoin and other assets. Flashnet powers any type of market with native Bitcoin settlement.

Learn more: [Flashnet Documentation](https://docs.flashnet.xyz/introduction)

## Chat Architecture

This bot is built on the principle of **chat separation** to provide flexible notification management. The architecture allows you to configure different chat channels for different types of information:

- **Main Chat (Big Sales Chat)**: Receives all large swap notifications above the configured BTC threshold. This is the primary channel for general market activity that everyone can see.

- **Filtered Chat**: Receives notifications only for specific tokens that you configure. This allows users who want more detailed and focused information to subscribe to a separate channel with filtered content.

**Example workflow:**
- The bot monitors all swap operations from Flashnet AMM API
- For each swap, it checks if it meets the criteria (BTC amount threshold, token type)
- If it's a general large swap → sends to **Main Chat** (visible to everyone)
- If it's a swap for a filtered token → sends to **Filtered Chat** (for users who want detailed info)

**Important notes:**
- Some commands (like `/flashadd`, `/flashdel`, `/flash`, `/flow`, `/stats`, `/spark`) work only in the **Filtered Chat**
- You decide which chat to use for your notifications based on your needs
- The main chat is for general market overview, while the filtered chat is for specific token tracking

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

This bot currently works with **GET requests** to fetch market data and monitor activity:

- **Flashnet API**: Main AMM swap data and authentication (GET requests for swaps, pools, history)
- **Luminex API**: Token metadata, holder information, wallet balances (GET requests for token data)

> 💡 *Note: This version focuses on monitoring and notifications. A future version might support POST requests for direct token swaps, limit orders, and active trading operations. Stay tuned! 😊*

## Development

### Building

```bash
go build ./cmd/bot/main.go
```

### Testing

The project includes integration tests for both Flashnet and Luminex APIs:

```bash
# Run all tests
go test ./...

# Run integration tests (requires API access)
go test -tags=integration ./internal/tests

# Run specific integration test
go test -tags=integration ./internal/tests -run TestIntegration_Flashnet
go test -tags=integration ./internal/tests -run TestIntegration_Luminex
```

**What we test:**
- Flashnet API authentication flow (challenge, signature, token)
- Flashnet API swap data retrieval
- Luminex API token metadata retrieval
- Luminex API wallet balance queries
- Error handling and retry mechanisms

**Example test output:**
```
ok      spark-wallet/internal/tests  2.345s
ok      spark-wallet/internal/clients_api/flashnet  0.123s
ok      spark-wallet/internal/clients_api/luminex  0.456s

=== Integration Tests ===
PASS: TestIntegration_Flashnet (1.234s)
  ✓ Authentication flow
  ✓ Get swaps endpoint
  ✓ Get pools endpoint

PASS: TestIntegration_Luminex (0.567s)
  ✓ Get token metadata
  ✓ Get wallet balance
```

### Code Quality

The project uses:
- `golangci-lint` for code analysis
- `.editorconfig` for consistent formatting

### Here are a few examples of tg notifications

![Preview](https://i.imgur.com/1ljUcid.png)
![Preview](https://i.imgur.com/1JogNXR.png)
![Preview](https://i.imgur.com/5goE57s.png)



