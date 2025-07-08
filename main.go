package main

import (
	"context"
	"log"
	"os"

	"tunnelman/models"
	"tunnelman/views"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
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
			} else if config.CloudflareZoneID != "" {
				if err := client.HealthCheck(ctx); err != nil {
					log.Printf("⚠️  Zone access issue: %v", err)
				} else {
					log.Println("✅ Cloudflare connected successfully!")
				}
			} else {
				log.Println("✅ Cloudflare connected (no zone ID configured)")
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
		log.Println("   - Zone:Zone:Read")
		log.Println("   - Zone:DNS:Edit") 
		log.Println("   - Account:Cloudflare Tunnel:Edit")
		log.Println("3. Add your domain to Zone Resources")
		log.Println("")
		
		if err := config.Save(); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}
		log.Printf("📝 Edit config: %s", models.GetConfigPath())
		log.Println("4. Add your token and zone ID to the config file")
		log.Println("5. Install cloudflared: brew install cloudflared")
		log.Println("6. Restart tunnelman")
		os.Exit(1)
	}

	model := views.NewModel(state, client, tunnelManager)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}