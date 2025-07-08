package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

type CloudflareClient struct {
	api       *cloudflare.API
	accountID string
	config    *Config
}

type TunnelResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type CLITunnel struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	CreatedAt   time.Time             `json:"created_at"`
	Connections []CLITunnelConnection `json:"conns,omitempty"`
}

type CLITunnelConnection struct {
	ColoName           string    `json:"colo_name"`
	ID                 string    `json:"id"`
	IsPendingReconnect bool      `json:"is_pending_reconnect"`
	OriginIP           string    `json:"origin_ip"`
	OpenedAt           time.Time `json:"opened_at"`
}

type TunnelConfigIngress struct {
	ID            string                 `json:"id,omitempty"`
	Hostname      string                 `json:"hostname,omitempty"`
	Path          string                 `json:"path,omitempty"`
	Service       string                 `json:"service"`
	OriginRequest map[string]interface{} `json:"originRequest,omitempty"`
}

type TunnelConfiguration struct {
	TunnelID  string           `json:"tunnel_id"`
	Version   int              `json:"version"`
	Config    TunnelConfigData `json:"config"`
	Source    string           `json:"source"`
	CreatedAt string           `json:"created_at"`
}

type TunnelConfigData struct {
	Ingress     []TunnelConfigIngress `json:"ingress"`
	WarpRouting WarpRouting           `json:"warp-routing"`
}

type WarpRouting struct {
	Enabled bool `json:"enabled"`
}

type PublicHostname struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Path     string `json:"path"`
	Service  string `json:"service"`
}

type DNSRecordRequest struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Proxied  *bool  `json:"proxied,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

func NewCloudflareClient(config *Config) (*CloudflareClient, error) {
	if config.CloudflareAPIKey == "" {
		return nil, fmt.Errorf("cloudflare API key is required")
	}

	var api *cloudflare.API
	var err error

	if config.CloudflareEmail != "" {
		api, err = cloudflare.New(config.CloudflareAPIKey, config.CloudflareEmail)
	} else {
		api, err = cloudflare.NewWithAPIToken(config.CloudflareAPIKey)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare API client: %w", err)
	}

	client := &CloudflareClient{
		api:    api,
		config: config,
	}

	ctx := context.Background()

	// Get account ID
	if accounts, _, err := api.Accounts(ctx, cloudflare.AccountsListParams{}); err == nil && len(accounts) > 0 {
		client.accountID = accounts[0].ID
	}

	return client, nil
}

func (c *CloudflareClient) ValidateCredentials(ctx context.Context) error {
	_, err := c.api.UserDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	return nil
}

func (c *CloudflareClient) GetZoneDomain() string {
	return ""
}

func (c *CloudflareClient) GetZoneID(ctx context.Context, domain string) (string, error) {
	zones, err := c.api.ListZones(ctx, domain)
	if err != nil {
		return "", fmt.Errorf("failed to list zones: %w", err)
	}

	if len(zones) == 0 {
		return "", fmt.Errorf("no zones found for domain: %s", domain)
	}

	return zones[0].ID, nil
}

func (c *CloudflareClient) ListAllZones(ctx context.Context) error {
	fmt.Printf("🔍 Listing all zones accessible to this token...\n")
	zones, err := c.api.ListZones(ctx)
	if err != nil {
		fmt.Printf("🔍 Failed to list zones: %v\n", err)
		return err
	}

	fmt.Printf("🔍 Found %d zones:\n", len(zones))
	for i, zone := range zones {
		fmt.Printf("  %d. Zone: %s (ID: %s)\n", i+1, zone.Name, zone.ID)
	}
	return nil
}

func (c *CloudflareClient) GetAvailableDomains(ctx context.Context) ([]string, error) {
	zones, err := c.api.ListZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}

	var domains []string
	for _, zone := range zones {
		domains = append(domains, zone.Name)
	}

	return domains, nil
}

// GetAccountID returns the account ID for the authenticated user
func (c *CloudflareClient) GetAccountID() string {
	return c.accountID
}

// Tunnel Management via CLI

func (c *CloudflareClient) execCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command failed: %s %v - %s", name, args, string(output))
	}
	return output, nil
}

func (c *CloudflareClient) ListTunnels(ctx context.Context) ([]CLITunnel, error) {
	output, err := c.execCommand("cloudflared", "tunnel", "--output", "json", "list")
	if err != nil {
		return nil, fmt.Errorf("failed to list tunnels: %w", err)
	}

	var tunnels []CLITunnel
	if err := json.Unmarshal(output, &tunnels); err != nil {
		return nil, fmt.Errorf("failed to parse tunnel list: %w", err)
	}

	return tunnels, nil
}

func (c *CloudflareClient) CreateTunnel(ctx context.Context, name string) (*CLITunnel, error) {
	output, err := c.execCommand("cloudflared", "tunnel", "--output", "json", "create", name)
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel: %w", err)
	}

	var tunnel CLITunnel
	if err := json.Unmarshal(output, &tunnel); err != nil {
		return nil, fmt.Errorf("failed to parse created tunnel: %w", err)
	}

	return &tunnel, nil
}

func (c *CloudflareClient) DeleteTunnel(ctx context.Context, nameOrID string) error {
	_, err := c.execCommand("cloudflared", "tunnel", "delete", nameOrID)
	if err != nil {
		return fmt.Errorf("failed to delete tunnel: %w", err)
	}
	return nil
}

func (c *CloudflareClient) GetTunnelInfo(ctx context.Context, nameOrID string) (*CLITunnel, error) {
	output, err := c.execCommand("cloudflared", "tunnel", "--output", "json", "info", nameOrID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnel info: %w", err)
	}

	var tunnel CLITunnel
	if err := json.Unmarshal(output, &tunnel); err != nil {
		return nil, fmt.Errorf("failed to parse tunnel info: %w", err)
	}

	return &tunnel, nil
}

func (c *CloudflareClient) StartTunnel(ctx context.Context, nameOrID, service string) error {
	args := []string{"tunnel", "--url", service, "run", nameOrID}

	cmd := exec.CommandContext(ctx, "cloudflared", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	return nil
}

func (c *CloudflareClient) StopTunnel(ctx context.Context, nameOrID string) error {
	cmd := exec.Command("pkill", "-f", fmt.Sprintf("cloudflared.*%s", nameOrID))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}
	return nil
}

func (c *CloudflareClient) RouteDNS(ctx context.Context, tunnelName, hostname string) error {
	_, err := c.execCommand("cloudflared", "tunnel", "route", "dns", tunnelName, hostname)
	if err != nil {
		return fmt.Errorf("failed to route DNS: %w", err)
	}
	return nil
}

func (c *CloudflareClient) CreateTunnelDNSRecord(ctx context.Context, tunnelName, hostname string, overwrite bool) error {
	args := []string{"tunnel", "route", "dns"}
	if overwrite {
		args = append(args, "--overwrite-dns")
	}
	args = append(args, tunnelName, hostname)

	_, err := c.execCommand("cloudflared", args...)
	if err != nil {
		return fmt.Errorf("failed to create tunnel DNS record: %w", err)
	}
	return nil
}

func (c *CloudflareClient) DeleteTunnelDNSRecord(ctx context.Context, hostname string) error {
	_, err := c.execCommand("cloudflared", "tunnel", "route", "dns", "delete", hostname)
	if err != nil {
		return fmt.Errorf("failed to delete tunnel DNS record: %w", err)
	}
	return nil
}

func (c *CloudflareClient) ValidateConfig(configPath string) error {
	args := []string{"tunnel", "ingress", "validate"}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	_, err := c.execCommand("cloudflared", args...)
	if err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}

// Tunnel Configuration Management via API

func (c *CloudflareClient) GetTunnelConfiguration(ctx context.Context, tunnelID string) (*TunnelConfiguration, error) {
	if c.accountID == "" {
		return nil, fmt.Errorf("account ID not available")
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", c.accountID, tunnelID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnel configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var response struct {
		Success bool                     `json:"success"`
		Result  TunnelConfiguration      `json:"result"`
		Errors  []map[string]interface{} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API request failed: %v", response.Errors)
	}

	return &response.Result, nil
}

func (c *CloudflareClient) UpdateTunnelConfiguration(ctx context.Context, tunnelID string, config *TunnelConfigData) error {
	if c.accountID == "" {
		return fmt.Errorf("account ID not available")
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", c.accountID, tunnelID)

	body, err := json.Marshal(map[string]interface{}{
		"config": config,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update tunnel configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var response struct {
		Success bool                     `json:"success"`
		Errors  []map[string]interface{} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("API request failed: %v", response.Errors)
	}

	return nil
}

func (c *CloudflareClient) GetPublicHostnames(ctx context.Context, tunnelID string) ([]PublicHostname, error) {
	config, err := c.GetTunnelConfiguration(ctx, tunnelID)
	if err != nil {
		return nil, err
	}

	var hostnames []PublicHostname
	for _, ingress := range config.Config.Ingress {
		// Skip catch-all rules without hostname
		if ingress.Hostname == "" {
			continue
		}

		hostnames = append(hostnames, PublicHostname{
			ID:       ingress.ID,
			Hostname: ingress.Hostname,
			Path:     ingress.Path,
			Service:  ingress.Service,
		})
	}

	return hostnames, nil
}

func (c *CloudflareClient) AddPublicHostname(ctx context.Context, tunnelID, hostname, path, service string) error {
	config, err := c.GetTunnelConfiguration(ctx, tunnelID)
	if err != nil {
		return err
	}

	// Set defaults
	if path == "" {
		path = "*"
	}
	if service == "" {
		service = "http://localhost:8080"
	}

	// Check if hostname already exists
	for _, ingress := range config.Config.Ingress {
		if ingress.Hostname == hostname && ingress.Path == path {
			return fmt.Errorf("hostname %s with path %s already exists", hostname, path)
		}
	}

	// Find the catch-all rule (should be last)
	var catchAllIndex = -1
	for i, ingress := range config.Config.Ingress {
		if ingress.Hostname == "" {
			catchAllIndex = i
			break
		}
	}

	// Generate a unique ID for the new ingress rule
	maxID := 0
	for _, ingress := range config.Config.Ingress {
		if ingress.ID != "" {
			if id, err := strconv.Atoi(ingress.ID); err == nil && id > maxID {
				maxID = id
			}
		}
	}

	newIngress := TunnelConfigIngress{
		ID:            strconv.Itoa(maxID + 1),
		Hostname:      hostname,
		Service:       service,
		OriginRequest: map[string]interface{}{},
	}

	// Only add path if it's not "*"
	if path != "*" {
		newIngress.Path = path
	}

	// Insert before catch-all rule
	if catchAllIndex >= 0 {
		config.Config.Ingress = append(config.Config.Ingress[:catchAllIndex],
			append([]TunnelConfigIngress{newIngress}, config.Config.Ingress[catchAllIndex:]...)...)
	} else {
		// No catch-all rule, append to end
		config.Config.Ingress = append(config.Config.Ingress, newIngress)
	}

	// Update the tunnel configuration
	if err := c.UpdateTunnelConfiguration(ctx, tunnelID, &config.Config); err != nil {
		return err
	}

	// Also create DNS record for the hostname
	if err := c.CreateTunnelDNSRecord(ctx, tunnelID, hostname, false); err != nil {
		// Log warning but don't fail the operation
		fmt.Printf("Warning: Failed to create DNS record for %s: %v\n", hostname, err)
	}

	return nil
}

func (c *CloudflareClient) UpdatePublicHostname(ctx context.Context, tunnelID, originalHostname, newHostname, path, service string) error {
	config, err := c.GetTunnelConfiguration(ctx, tunnelID)
	if err != nil {
		return err
	}

	// Find the ingress rule to update
	var ingressToUpdate *TunnelConfigIngress
	for i := range config.Config.Ingress {
		if config.Config.Ingress[i].Hostname == originalHostname {
			ingressToUpdate = &config.Config.Ingress[i]
			break
		}
	}

	if ingressToUpdate == nil {
		return fmt.Errorf("hostname %s not found", originalHostname)
	}

	// Set defaults
	if path == "" {
		path = "*"
	}
	if service == "" {
		service = "http://localhost:8080"
	}

	// Update the fields
	ingressToUpdate.Hostname = newHostname
	ingressToUpdate.Service = service

	// Only set path if it's not "*"
	if path != "*" {
		ingressToUpdate.Path = path
	} else {
		ingressToUpdate.Path = ""
	}

	// Update the tunnel configuration
	return c.UpdateTunnelConfiguration(ctx, tunnelID, &config.Config)
}

func (c *CloudflareClient) RemovePublicHostname(ctx context.Context, tunnelID, hostname, path string) error {
	config, err := c.GetTunnelConfiguration(ctx, tunnelID)
	if err != nil {
		return err
	}

	// Set default path if empty
	if path == "" {
		path = "*"
	}

	// Find and remove the ingress rule
	found := false
	for i, ingress := range config.Config.Ingress {
		// Handle path matching: empty path in config means "*"
		ingressPath := ingress.Path
		if ingressPath == "" {
			ingressPath = "*"
		}
		if ingress.Hostname == hostname && ingressPath == path {
			config.Config.Ingress = append(config.Config.Ingress[:i], config.Config.Ingress[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("hostname %s with path %s not found", hostname, path)
	}

	return c.UpdateTunnelConfiguration(ctx, tunnelID, &config.Config)
}

// Status and Monitoring

func (c *CloudflareClient) GetTunnelStatus(ctx context.Context, nameOrID string) (TunnelStatus, error) {
	tunnel, err := c.GetTunnelInfo(ctx, nameOrID)
	if err != nil {
		return StatusUnknown, err
	}

	if len(tunnel.Connections) == 0 {
		return StatusInactive, nil
	}

	hasActiveConnection := false
	for _, conn := range tunnel.Connections {
		if !conn.IsPendingReconnect {
			hasActiveConnection = true
			break
		}
	}

	if hasActiveConnection {
		return StatusActive, nil
	}

	return StatusInactive, nil
}

func (c *CloudflareClient) HealthCheck(ctx context.Context) error {
	if err := c.ValidateCredentials(ctx); err != nil {
		return fmt.Errorf("credentials validation failed: %w", err)
	}

	return nil
}

// Utility Methods

func (c *CloudflareClient) IsCloudflaredInstalled() bool {
	_, err := exec.LookPath("cloudflared")
	return err == nil
}

func (c *CloudflareClient) GetCloudflaredVersion() (string, error) {
	output, err := c.execCommand("cloudflared", "--version")
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

func (c *CloudflareClient) CreateTunnelResponse(success bool, message string, data interface{}) TunnelResponse {
	return TunnelResponse{
		Success: success,
		Message: message,
		Data:    data,
	}
}

// Error Handling Helpers

type CloudflareError struct {
	Operation string
	Err       error
	Details   string
}

func (e CloudflareError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s failed: %v - %s", e.Operation, e.Err, e.Details)
	}
	return fmt.Sprintf("%s failed: %v", e.Operation, e.Err)
}

func (c *CloudflareClient) wrapError(operation string, err error, details string) error {
	if err == nil {
		return nil
	}
	return CloudflareError{
		Operation: operation,
		Err:       err,
		Details:   details,
	}
}

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "does not exist")
}

func IsAuthenticationError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "authentication") ||
		strings.Contains(err.Error(), "unauthorized") ||
		strings.Contains(err.Error(), "invalid token")
}

func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "rate limit") ||
		strings.Contains(err.Error(), "too many requests")
}
