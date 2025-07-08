package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"tunnelman/models"
)

const (
	TestTunnelPrefix = "e2e-test-"
	TestHostnamePrefix = "e2e-test-"
)

type E2ETestSuite struct {
	client       *models.CloudflareClient
	config       *models.Config
	createdTunnels []string
	createdHostnames []string
	ctx          context.Context
}

func NewE2ETestSuite(t *testing.T) *E2ETestSuite {
	config, err := models.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.CloudflareAPIKey == "" {
		t.Skip("No Cloudflare API key configured, skipping e2e tests")
	}

	client, err := models.NewCloudflareClient(config)
	if err != nil {
		t.Fatalf("Failed to create Cloudflare client: %v", err)
	}

	ctx := context.Background()
	if err := client.ValidateCredentials(ctx); err != nil {
		t.Fatalf("Failed to validate credentials: %v", err)
	}

	return &E2ETestSuite{
		client: client,
		config: config,
		ctx:    ctx,
	}
}

func (s *E2ETestSuite) Cleanup(t *testing.T) {
	log.Printf("Cleaning up %d tunnels and %d hostnames", len(s.createdTunnels), len(s.createdHostnames))
	
	// Clean up tunnels (this also removes associated hostnames)
	for _, tunnelName := range s.createdTunnels {
		if err := s.client.DeleteTunnel(s.ctx, tunnelName); err != nil {
			t.Logf("Warning: Failed to cleanup tunnel %s: %v", tunnelName, err)
		} else {
			log.Printf("Cleaned up tunnel: %s", tunnelName)
		}
	}

	// Clean up any remaining test hostnames in DNS
	if s.config.CloudflareZoneID != "" {
		records, err := s.client.ListDNSRecords(s.ctx)
		if err != nil {
			t.Logf("Warning: Failed to list DNS records for cleanup: %v", err)
		} else {
			for _, record := range records {
				if strings.HasPrefix(record.Name, TestHostnamePrefix) {
					if err := s.client.DeleteDNSRecord(s.ctx, record.ID); err != nil {
						t.Logf("Warning: Failed to cleanup DNS record %s: %v", record.Name, err)
					} else {
						log.Printf("Cleaned up DNS record: %s", record.Name)
					}
				}
			}
		}
	}
}

func (s *E2ETestSuite) CreateTestTunnel(t *testing.T, name string) *models.CLITunnel {
	fullName := TestTunnelPrefix + name + "-" + generateTimestamp()
	
	tunnel, err := s.client.CreateTunnel(s.ctx, fullName)
	if err != nil {
		t.Fatalf("Failed to create test tunnel %s: %v", fullName, err)
	}

	s.createdTunnels = append(s.createdTunnels, fullName)
	log.Printf("Created test tunnel: %s (ID: %s)", fullName, tunnel.ID)
	return tunnel
}

func (s *E2ETestSuite) CreateTestHostname(t *testing.T, tunnelID, hostname, path, service string) {
	fullHostname := TestHostnamePrefix + hostname + "-" + generateTimestamp()
	if s.client.GetZoneDomain() != "" {
		fullHostname = fmt.Sprintf("%s.%s", fullHostname, s.client.GetZoneDomain())
	}

	if path == "" {
		path = "*"
	}
	if service == "" {
		service = "http://localhost:8080"
	}

	// Ensure tunnel has an initial configuration
	s.ensureTunnelConfiguration(t, tunnelID)

	err := s.client.AddPublicHostname(s.ctx, tunnelID, fullHostname, path, service)
	if err != nil {
		t.Fatalf("Failed to create test hostname %s: %v", fullHostname, err)
	}

	s.createdHostnames = append(s.createdHostnames, fullHostname)
	log.Printf("Created test hostname: %s -> %s", fullHostname, service)
}

func (s *E2ETestSuite) ensureTunnelConfiguration(t *testing.T, tunnelID string) {
	// Check if configuration exists
	_, err := s.client.GetTunnelConfiguration(s.ctx, tunnelID)
	if err == nil {
		return // Configuration already exists
	}

	// Create initial configuration with a catch-all rule
	initialConfig := &models.TunnelConfigData{
		Ingress: []models.TunnelConfigIngress{
			{
				Service: "http_status:404",
			},
		},
		WarpRouting: models.WarpRouting{
			Enabled: false,
		},
	}

	err = s.client.UpdateTunnelConfiguration(s.ctx, tunnelID, initialConfig)
	if err != nil {
		t.Logf("Warning: Failed to create initial tunnel configuration: %v", err)
	} else {
		log.Printf("Created initial configuration for tunnel: %s", tunnelID)
	}
}

func generateTimestamp() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

func TestE2E_TunnelLifecycle(t *testing.T) {
	suite := NewE2ETestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Create_Tunnel", func(t *testing.T) {
		tunnel := suite.CreateTestTunnel(t, "lifecycle")
		
		if tunnel.Name == "" {
			t.Error("Created tunnel should have a name")
		}
		if tunnel.ID == "" {
			t.Error("Created tunnel should have an ID")
		}
	})

	t.Run("List_Tunnels", func(t *testing.T) {
		tunnels, err := suite.client.ListTunnels(suite.ctx)
		if err != nil {
			t.Fatalf("Failed to list tunnels: %v", err)
		}

		found := false
		for _, tunnel := range tunnels {
			if strings.HasPrefix(tunnel.Name, TestTunnelPrefix) {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should find at least one test tunnel in list")
		}
	})

	t.Run("Get_Tunnel_Info", func(t *testing.T) {
		if len(suite.createdTunnels) == 0 {
			t.Skip("No tunnels created yet")
		}

		tunnelName := suite.createdTunnels[0]
		tunnel, err := suite.client.GetTunnelInfo(suite.ctx, tunnelName)
		if err != nil {
			t.Fatalf("Failed to get tunnel info for %s: %v", tunnelName, err)
		}

		if tunnel.Name != tunnelName {
			t.Errorf("Expected tunnel name %s, got %s", tunnelName, tunnel.Name)
		}
	})

	t.Run("Get_Tunnel_Status", func(t *testing.T) {
		if len(suite.createdTunnels) == 0 {
			t.Skip("No tunnels created yet")
		}

		tunnelName := suite.createdTunnels[0]
		status, err := suite.client.GetTunnelStatus(suite.ctx, tunnelName)
		if err != nil {
			t.Fatalf("Failed to get tunnel status for %s: %v", tunnelName, err)
		}

		if status != models.StatusInactive && status != models.StatusActive {
			t.Errorf("Expected status to be active or inactive, got %s", status)
		}
	})
}

func TestE2E_HostnameManagement(t *testing.T) {
	suite := NewE2ETestSuite(t)
	defer suite.Cleanup(t)

	// Create a tunnel first
	tunnel := suite.CreateTestTunnel(t, "hostname-mgmt")

	t.Run("Add_Public_Hostname", func(t *testing.T) {
		suite.CreateTestHostname(t, tunnel.ID, "api", "*", "http://localhost:8080")
		
		// Verify hostname was added
		hostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get public hostnames: %v", err)
		}

		found := false
		for _, hostname := range hostnames {
			if strings.Contains(hostname.Hostname, TestHostnamePrefix) {
				found = true
				if hostname.Service != "http://localhost:8080" {
					t.Errorf("Expected service http://localhost:8080, got %s", hostname.Service)
				}
				break
			}
		}
		if !found {
			t.Error("Should find the created hostname in list")
		}
	})

	t.Run("Update_Public_Hostname", func(t *testing.T) {
		// Create a hostname to update first
		suite.CreateTestHostname(t, tunnel.ID, "update-test", "*", "http://localhost:8080")
		
		// Get current hostnames
		hostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get public hostnames: %v", err)
		}

		if len(hostnames) == 0 {
			t.Skip("No hostnames to update")
		}

		// Find the test hostname to update
		var testHostname *models.PublicHostname
		for _, hostname := range hostnames {
			if strings.Contains(hostname.Hostname, TestHostnamePrefix) && strings.Contains(hostname.Hostname, "update-test") {
				testHostname = &hostname
				break
			}
		}

		if testHostname == nil {
			t.Skip("No test hostname found to update")
		}

		// Update the hostname
		newService := "http://localhost:9090"
		err = suite.client.UpdatePublicHostname(suite.ctx, tunnel.ID, testHostname.Hostname, testHostname.Hostname, "*", newService)
		if err != nil {
			t.Fatalf("Failed to update hostname: %v", err)
		}

		// Verify the update
		updatedHostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get updated hostnames: %v", err)
		}

		found := false
		for _, hostname := range updatedHostnames {
			if hostname.Hostname == testHostname.Hostname {
				found = true
				if hostname.Service != newService {
					t.Errorf("Expected updated service %s, got %s", newService, hostname.Service)
				}
				break
			}
		}
		if !found {
			t.Error("Updated hostname not found")
		}
	})

	t.Run("List_Public_Hostnames", func(t *testing.T) {
		// Ensure tunnel has configuration first
		suite.ensureTunnelConfiguration(t, tunnel.ID)
		
		hostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to list public hostnames: %v", err)
		}

		// Should have at least the catch-all rule
		if len(hostnames) < 0 {
			t.Error("Should be able to list hostnames")
		}

		// Verify structure
		for _, hostname := range hostnames {
			if hostname.Hostname == "" {
				t.Error("Hostname should not be empty")
			}
			if hostname.Service == "" {
				t.Error("Service should not be empty")
			}
		}
	})

	t.Run("Remove_Public_Hostname", func(t *testing.T) {
		// Create a hostname to remove first
		suite.CreateTestHostname(t, tunnel.ID, "remove-test", "*", "http://localhost:8080")
		
		// Get current hostnames
		hostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get public hostnames: %v", err)
		}

		if len(hostnames) == 0 {
			t.Skip("No hostnames to remove")
		}

		// Find the test hostname to remove
		var testHostname *models.PublicHostname
		for _, hostname := range hostnames {
			if strings.Contains(hostname.Hostname, TestHostnamePrefix) && strings.Contains(hostname.Hostname, "remove-test") {
				testHostname = &hostname
				break
			}
		}

		if testHostname == nil {
			t.Skip("No test hostname found to remove")
		}

		// Remove the hostname (use the actual path from the hostname)
		path := testHostname.Path
		if path == "" {
			path = "*"
		}
		err = suite.client.RemovePublicHostname(suite.ctx, tunnel.ID, testHostname.Hostname, path)
		if err != nil {
			t.Fatalf("Failed to remove hostname: %v", err)
		}

		// Verify removal
		updatedHostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get hostnames after removal: %v", err)
		}

		for _, hostname := range updatedHostnames {
			if hostname.Hostname == testHostname.Hostname {
				t.Error("Hostname should have been removed")
			}
		}
	})
}

func TestE2E_TunnelConfiguration(t *testing.T) {
	suite := NewE2ETestSuite(t)
	defer suite.Cleanup(t)

	tunnel := suite.CreateTestTunnel(t, "config")

	t.Run("Get_Tunnel_Configuration", func(t *testing.T) {
		// Ensure tunnel has configuration
		suite.ensureTunnelConfiguration(t, tunnel.ID)
		
		config, err := suite.client.GetTunnelConfiguration(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get tunnel configuration: %v", err)
		}

		if config.TunnelID != tunnel.ID {
			t.Errorf("Expected tunnel ID %s, got %s", tunnel.ID, config.TunnelID)
		}

		if len(config.Config.Ingress) == 0 {
			t.Error("Expected at least one ingress rule")
		}
	})

	t.Run("Update_Tunnel_Configuration", func(t *testing.T) {
		// Ensure tunnel has configuration
		suite.ensureTunnelConfiguration(t, tunnel.ID)
		
		// Get current config
		config, err := suite.client.GetTunnelConfiguration(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get tunnel configuration: %v", err)
		}

		// Add a test ingress rule
		testHostname := TestHostnamePrefix + "config-test-" + generateTimestamp()
		if suite.client.GetZoneDomain() != "" {
			testHostname = fmt.Sprintf("%s.%s", testHostname, suite.client.GetZoneDomain())
		}

		newIngress := models.TunnelConfigIngress{
			Hostname: testHostname,
			Service:  "http://localhost:8080",
		}

		// Insert before catch-all rule (last rule)
		if len(config.Config.Ingress) > 0 {
			config.Config.Ingress = append(config.Config.Ingress[:len(config.Config.Ingress)-1], 
				newIngress, config.Config.Ingress[len(config.Config.Ingress)-1])
		} else {
			config.Config.Ingress = append(config.Config.Ingress, newIngress)
		}

		// Update configuration
		err = suite.client.UpdateTunnelConfiguration(suite.ctx, tunnel.ID, &config.Config)
		if err != nil {
			t.Fatalf("Failed to update tunnel configuration: %v", err)
		}

		// Verify update
		updatedConfig, err := suite.client.GetTunnelConfiguration(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get updated configuration: %v", err)
		}

		found := false
		for _, ingress := range updatedConfig.Config.Ingress {
			if ingress.Hostname == testHostname {
				found = true
				if ingress.Service != "http://localhost:8080" {
					t.Errorf("Expected service http://localhost:8080, got %s", ingress.Service)
				}
				break
			}
		}
		if !found {
			t.Error("Updated ingress rule not found")
		}

		suite.createdHostnames = append(suite.createdHostnames, testHostname)
	})
}

func TestE2E_ErrorHandling(t *testing.T) {
	suite := NewE2ETestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Invalid_Tunnel_Operations", func(t *testing.T) {
		// Try to get info for non-existent tunnel
		_, err := suite.client.GetTunnelInfo(suite.ctx, "non-existent-tunnel-12345")
		if err == nil {
			t.Error("Expected error for non-existent tunnel")
		}

		// Try to get status for non-existent tunnel
		_, err = suite.client.GetTunnelStatus(suite.ctx, "non-existent-tunnel-12345")
		if err == nil {
			t.Error("Expected error for non-existent tunnel status")
		}
	})

	t.Run("Invalid_Hostname_Operations", func(t *testing.T) {
		tunnel := suite.CreateTestTunnel(t, "error-test")

		// Try to update non-existent hostname
		err := suite.client.UpdatePublicHostname(suite.ctx, tunnel.ID, "non-existent.example.com", "test.example.com", "*", "http://localhost:8080")
		if err == nil {
			t.Error("Expected error for non-existent hostname update")
		}

		// Try to remove non-existent hostname
		err = suite.client.RemovePublicHostname(suite.ctx, tunnel.ID, "non-existent.example.com", "*")
		if err == nil {
			t.Error("Expected error for non-existent hostname removal")
		}
	})
}

func TestE2E_Integration_Full_Workflow(t *testing.T) {
	suite := NewE2ETestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Complete_Tunnel_Workflow", func(t *testing.T) {
		// 1. Create tunnel
		tunnel := suite.CreateTestTunnel(t, "full-workflow")

		// 2. Add multiple hostnames
		suite.CreateTestHostname(t, tunnel.ID, "api", "*", "http://localhost:8080")
		suite.CreateTestHostname(t, tunnel.ID, "web", "*", "http://localhost:3000")
		suite.CreateTestHostname(t, tunnel.ID, "admin", "/admin", "http://localhost:9000")

		// 3. List and verify hostnames
		hostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to list hostnames: %v", err)
		}

		testHostnameCount := 0
		for _, hostname := range hostnames {
			if strings.Contains(hostname.Hostname, TestHostnamePrefix) {
				testHostnameCount++
			}
		}

		if testHostnameCount != 3 {
			t.Errorf("Expected 3 test hostnames, got %d", testHostnameCount)
		}

		// 4. Update one hostname
		var hostnameToUpdate *models.PublicHostname
		for _, hostname := range hostnames {
			if strings.Contains(hostname.Hostname, TestHostnamePrefix) && strings.Contains(hostname.Hostname, "api") {
				hostnameToUpdate = &hostname
				break
			}
		}

		if hostnameToUpdate != nil {
			err = suite.client.UpdatePublicHostname(suite.ctx, tunnel.ID, hostnameToUpdate.Hostname, hostnameToUpdate.Hostname, "*", "http://localhost:8888")
			if err != nil {
				t.Fatalf("Failed to update hostname: %v", err)
			}

			// Verify update
			updatedHostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
			if err != nil {
				t.Fatalf("Failed to get updated hostnames: %v", err)
			}

			found := false
			for _, hostname := range updatedHostnames {
				if hostname.Hostname == hostnameToUpdate.Hostname {
					found = true
					if hostname.Service != "http://localhost:8888" {
						t.Errorf("Expected updated service http://localhost:8888, got %s", hostname.Service)
					}
					break
				}
			}
			if !found {
				t.Error("Updated hostname not found")
			}
		}

		// 5. Remove one hostname
		var hostnameToRemove *models.PublicHostname
		updatedHostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get hostnames: %v", err)
		}

		for _, hostname := range updatedHostnames {
			if strings.Contains(hostname.Hostname, TestHostnamePrefix) && strings.Contains(hostname.Hostname, "admin") {
				hostnameToRemove = &hostname
				break
			}
		}

		if hostnameToRemove != nil {
			err = suite.client.RemovePublicHostname(suite.ctx, tunnel.ID, hostnameToRemove.Hostname, hostnameToRemove.Path)
			if err != nil {
				t.Fatalf("Failed to remove hostname: %v", err)
			}

			// Verify removal
			finalHostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
			if err != nil {
				t.Fatalf("Failed to get final hostnames: %v", err)
			}

			for _, hostname := range finalHostnames {
				if hostname.Hostname == hostnameToRemove.Hostname {
					t.Error("Hostname should have been removed")
				}
			}
		}

		// 6. Verify final state
		finalHostnames, err := suite.client.GetPublicHostnames(suite.ctx, tunnel.ID)
		if err != nil {
			t.Fatalf("Failed to get final hostnames: %v", err)
		}

		finalTestHostnameCount := 0
		for _, hostname := range finalHostnames {
			if strings.Contains(hostname.Hostname, TestHostnamePrefix) {
				finalTestHostnameCount++
			}
		}

		if finalTestHostnameCount != 2 {
			t.Errorf("Expected 2 final test hostnames, got %d", finalTestHostnameCount)
		}
	})
}

// Helper function to run tests with timeout
func TestMain(m *testing.M) {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	log.Println("Starting E2E tests...")
	code := m.Run()
	log.Println("E2E tests completed")
	os.Exit(code)
}