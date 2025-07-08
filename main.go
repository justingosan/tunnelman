package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"tunnelman/models"
	"tunnelman/views"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func promptUser(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func setupInteractiveConfig() (*models.Config, error) {
	fmt.Println("🔧 Interactive Configuration Setup")
	fmt.Println("")

	// Get API Token
	fmt.Println("1. First, get your Cloudflare API Token:")
	fmt.Println("   Visit: https://dash.cloudflare.com/profile/api-tokens")
	fmt.Println("   Create Token → Custom Token → Set permissions:")
	fmt.Println("   - Account:Cloudflare Tunnel:Edit")
	fmt.Println("")

	apiKey := promptUser("Enter your Cloudflare API Token: ")
	if apiKey == "" {
		return nil, fmt.Errorf("API token is required")
	}

	// Optional email
	fmt.Println("")
	fmt.Println("2. Email address (optional, leave blank if using API token):")
	email := promptUser("Enter your Cloudflare email (optional): ")

	// Create config
	config := &models.Config{
		CloudflareAPIKey:   apiKey,
		CloudflareEmail:    email,
		TunnelConfigPath:   "",
		AutoRefreshSeconds: 30,
		LogLevel:           "info",
	}

	// Apply defaults
	defaultConfig := models.DefaultConfig()
	if config.TunnelConfigPath == "" {
		config.TunnelConfigPath = defaultConfig.TunnelConfigPath
	}

	fmt.Println("")
	fmt.Printf("💾 Saving configuration to: %s\n", models.GetConfigPath())

	if err := config.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✅ Configuration saved successfully!")
	fmt.Println("")

	return config, nil
}

func runConfigCommand() {
	fmt.Println("🔧 Tunnelman Configuration")
	fmt.Println("")

	// Show current config if it exists
	if config, err := models.LoadConfig(); err == nil && config.CloudflareAPIKey != "" {
		fmt.Printf("📋 Current configuration: %s\n", models.GetConfigPath())
		fmt.Printf("   API Key: %s***\n", config.CloudflareAPIKey[:min(8, len(config.CloudflareAPIKey))])
		if config.CloudflareEmail != "" {
			fmt.Printf("   Email: %s\n", config.CloudflareEmail)
		}
		fmt.Printf("   Auto Refresh: %d seconds\n", config.AutoRefreshSeconds)
		fmt.Printf("   Log Level: %s\n", config.LogLevel)
		fmt.Println("")

		response := promptUser("Do you want to reconfigure? (y/N): ")
		if !strings.HasPrefix(strings.ToLower(response), "y") {
			fmt.Println("Configuration unchanged.")
			return
		}
		fmt.Println("")
	}

	// Run interactive setup
	if _, err := setupInteractiveConfig(); err != nil {
		log.Fatalf("Configuration failed: %v", err)
	}

	fmt.Println("🎉 Configuration complete! You can now run 'tunnelman' to start the TUI.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	versionFlag := flag.Bool("version", false, "Show version information")
	helpFlag := flag.Bool("help", false, "Show help information")
	flag.Parse()

	// Check for subcommands
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "config":
			runConfigCommand()
			return
		default:
			fmt.Printf("Unknown command: %s\n", args[0])
			fmt.Println("Available commands: config")
			os.Exit(1)
		}
	}

	if *helpFlag {
		fmt.Println("tunnelman - Terminal User Interface for managing Cloudflare Tunnels")
		fmt.Printf("Version: %s\n\n", version)
		fmt.Println("Usage:")
		fmt.Println("  tunnelman [options]")
		fmt.Println("  tunnelman config         Interactive configuration setup")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -help      Show this help information")
		fmt.Println("  -version   Show version information")
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("  Run 'tunnelman config' for interactive setup")
		fmt.Println("  Or edit ~/.tunnelman/config.json manually")
		fmt.Println()
		fmt.Println("For more information, visit: https://github.com/justingosan/tunnelman")
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("tunnelman version %s\n", version)
		os.Exit(0)
	}
	config, err := models.LoadConfig()
	if err != nil {
		log.Printf("Warning: Failed to load config: %v", err)
		config = models.DefaultConfig()
	}

	// Check if we need to prompt for configuration
	needsConfig := config.CloudflareAPIKey == ""

	var client *models.CloudflareClient
	if config.CloudflareAPIKey != "" {
		client, err = models.NewCloudflareClient(config)
		if err != nil {
			log.Printf("❌ Failed to create Cloudflare client: %v", err)
			needsConfig = true
		} else {
			ctx := context.Background()
			if err := client.ValidateCredentials(ctx); err != nil {
				log.Printf("❌ Authentication failed: %v", err)
				needsConfig = true
			} else {
				log.Println("✅ Cloudflare connected successfully!")
			}
		}
	}

	// If config is needed, prompt for interactive setup
	if needsConfig {
		fmt.Println("")
		fmt.Println("🔧 Configuration required to proceed.")
		fmt.Println("")

		response := promptUser("Would you like to set up configuration now? (Y/n): ")
		if strings.ToLower(response) == "n" {
			fmt.Println("")
			fmt.Println("You can run 'tunnelman config' later to set up configuration.")
			fmt.Println("Or manually edit: " + models.GetConfigPath())
			os.Exit(0)
		}

		// Run interactive setup
		newConfig, err := setupInteractiveConfig()
		if err != nil {
			log.Fatalf("Configuration failed: %v", err)
		}
		config = newConfig

		// Try to create client with new config
		client, err = models.NewCloudflareClient(config)
		if err != nil {
			log.Fatalf("❌ Failed to create Cloudflare client with new config: %v", err)
		}

		ctx := context.Background()
		if err := client.ValidateCredentials(ctx); err != nil {
			log.Fatalf("❌ Authentication failed with new config: %v", err)
		}

		fmt.Println("✅ Cloudflare connected successfully!")
		fmt.Println("🚀 Starting Tunnelman...")
		fmt.Println("")
	}

	state := models.NewAppState()
	tunnelManager := models.NewTunnelManager(client, "")

	model := views.NewModel(state, client, tunnelManager)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
