package commands

// Root command for Cobra CLI
// Defines the main command structure of the application
// Registers all subcommands (bot, big-sales, holders, auth)

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "flashnet-api",
	Short: "Flashnet Market Monitor - Telegram bot for monitoring Flashnet/Spark AMM activity",
	Long: `Flashnet Market Monitor is a Go-based Telegram bot for monitoring Flashnet/Spark AMM activity 
with real-time notifications, chart generation, and comprehensive market analytics.`,
	Version: "1.0.0",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(botCmd)
	rootCmd.AddCommand(bigSalesCmd)
	rootCmd.AddCommand(holdersCmd)
	rootCmd.AddCommand(authCmd)
}

