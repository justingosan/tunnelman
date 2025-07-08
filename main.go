package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"tunnelman/models"
	"tunnelman/views"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "Show version information")
	helpFlag := flag.Bool("help", false, "Show help information")
	flag.Parse()

	if *helpFlag {
		fmt.Println("tunnelman - Terminal User Interface for managing Cloudflare Tunnels")
		fmt.Printf("Version: %s\n\n", version)
		fmt.Println("Usage:")
		fmt.Println("  tunnelman [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -help      Show this help information")
		fmt.Println("  -version   Show version information")
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("  Edit ~/.tunnelman/config.json with your Cloudflare API credentials")
		fmt.Println()
		fmt.Println("For more information, visit: https://github.com/your-username/tunnelman")
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

	var client *models.CloudflareClient
	if config.CloudflareAPIKey != "" {
		client, err = models.NewCloudflareClient(config)
		if err != nil {
			log.Printf("❌ Failed to create Cloudflare client: %v", err)
			client = nil
		} else {
			ctx := context.Background()
			if err := client.ValidateCredentials(ctx); err != nil {
				log.Printf("❌ Authentication failed: %v", err)
				log.Printf("📝 Check your credentials in: %s", models.GetConfigPath())
				client = nil
			} else {
				log.Println("✅ Cloudflare connected successfully!")
			}
		}
	}

	state := models.NewAppState()
	tunnelManager := models.NewTunnelManager(client, "")

	if client == nil {
		log.Println("🔑 Cloudflare credentials needed")
		log.Println("")
		log.Println("1. Get API Token: https://dash.cloudflare.com/profile/api-tokens")
		log.Println("2. Create Token → Custom Token → Set permissions:")
		log.Println("   - Account:Cloudflare Tunnel:Edit")
		log.Println("")

		if err := config.Save(); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}
		log.Printf("📝 Edit config: %s", models.GetConfigPath())
		log.Println("3. Add your token to the config file")
		log.Println("4. Install cloudflared: brew install cloudflared")
		log.Println("5. Restart tunnelman")
		os.Exit(1)
	}

	model := views.NewModel(state, client, tunnelManager)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
