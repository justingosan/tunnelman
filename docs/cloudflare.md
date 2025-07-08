# Cloudflare CLI Reference for Tunnelman

This document provides a comprehensive reference for Cloudflare CLI commands and API endpoints needed for the Tunnelman TUI application.

## Prerequisites

Before using Cloudflare Tunnel, ensure you have:

1. **Add a website to Cloudflare** - Your domain must be managed by Cloudflare
2. **Change your domain nameservers to Cloudflare** - Required for DNS management
3. **Install cloudflared** - The Cloudflare Tunnel daemon

## Installation

### Install cloudflared

Download cloudflared from the [releases page](https://github.com/cloudflare/cloudflared/releases) or install via package manager:

```bash
# macOS (Homebrew)
brew install cloudflared

# Ubuntu/Debian
wget https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
sudo dpkg -i cloudflared-linux-amd64.deb

# Windows
# Download cloudflared.exe from GitHub releases
```

## Authentication

### Login to Cloudflare

```bash
cloudflared tunnel login
```

This command:
- Opens a browser window for Cloudflare login
- Prompts to select your hostname
- Generates a certificate file (`cert.pem`) in the default cloudflared directory

### Authentication Files

- **Location**: `~/.cloudflared/` (default directory)
- **Certificate**: `cert.pem` - Account certificate for API access
- **Config**: `config.yml` - Tunnel configuration file

## Tunnel Management Commands

### Create a Tunnel

```bash
# Create a new tunnel
cloudflared tunnel create <TUNNEL_NAME>

# Example
cloudflared tunnel create my-tunnel
```

**Output**: Creates a tunnel and returns the tunnel ID (UUID)

### List Tunnels

```bash
# List all tunnels
cloudflared tunnel list

# List tunnels with detailed info
cloudflared tunnel list --output json
```

### Delete a Tunnel

```bash
# Delete a tunnel by name
cloudflared tunnel delete <TUNNEL_NAME>

# Delete a tunnel by ID
cloudflared tunnel delete <TUNNEL_ID>

# Force delete (cleanup DNS records)
cloudflared tunnel delete --force <TUNNEL_NAME>
```

### Run a Tunnel

```bash
# Run tunnel with configuration file
cloudflared tunnel run <TUNNEL_NAME>

# Run tunnel with inline configuration
cloudflared tunnel --url http://localhost:8080 run <TUNNEL_NAME>

# Run tunnel as background service
cloudflared tunnel --url http://localhost:8080 run <TUNNEL_NAME> &
```

### Tunnel Status and Information

```bash
# Get tunnel information
cloudflared tunnel info <TUNNEL_NAME>

# Check tunnel status
cloudflared tunnel list | grep <TUNNEL_NAME>

# View tunnel connections
cloudflared tunnel info <TUNNEL_NAME> --output json
```

## Configuration Management

### Configuration File Structure

Default location: `~/.cloudflared/config.yml`

```yaml
tunnel: <TUNNEL_ID>
credentials-file: /path/to/credentials.json

ingress:
  - hostname: example.com
    service: http://localhost:8080
  - hostname: api.example.com
    service: http://localhost:3000
  - service: http_status:404
```

### Configuration Commands

```bash
# Validate configuration
cloudflared tunnel ingress validate

# Test configuration rules
cloudflared tunnel ingress rule https://example.com

# Show effective configuration
cloudflared tunnel ingress validate --output json
```

## DNS Management

### Route DNS to Tunnel

```bash
# Route DNS to tunnel
cloudflared tunnel route dns <TUNNEL_NAME> <HOSTNAME>

# Example
cloudflared tunnel route dns my-tunnel example.com
cloudflared tunnel route dns my-tunnel api.example.com
```

### List DNS Routes

```bash
# List all DNS routes for tunnels
cloudflared tunnel route dns list

# List routes for specific tunnel
cloudflared tunnel route dns list --tunnel <TUNNEL_NAME>
```

### Delete DNS Routes

```bash
# Delete DNS route
cloudflared tunnel route dns delete <HOSTNAME>

# Example
cloudflared tunnel route dns delete example.com
```

## Access Control

### Access Commands

```bash
# Access help
cloudflared access help

# Access specific service
cloudflared access <service-url>

# TCP access
cloudflared access tcp --hostname <hostname> --url <local-url>
```

## Service Management

### Install as System Service

```bash
# Install service
cloudflared service install

# Start service
cloudflared service start

# Stop service
cloudflared service stop

# Uninstall service
cloudflared service uninstall
```

### Service Status

```bash
# Check service status (Linux)
systemctl status cloudflared

# Check service status (macOS)
launchctl list | grep cloudflared

# Check service status (Windows)
sc query cloudflared
```

## Logging and Debugging

### Log Commands

```bash
# Run with verbose logging
cloudflared tunnel --loglevel debug run <TUNNEL_NAME>

# Run with JSON logging
cloudflared tunnel --log-format json run <TUNNEL_NAME>

# Log to file
cloudflared tunnel --log-file /var/log/cloudflared.log run <TUNNEL_NAME>
```

### Debug Information

```bash
# Show version
cloudflared --version

# Show help
cloudflared tunnel help

# Show ingress rules
cloudflared tunnel ingress validate
```

## API Endpoints (for Go SDK)

### Tunnel Management

```go
// List tunnels
GET /accounts/{account_id}/cfd_tunnel

// Create tunnel
POST /accounts/{account_id}/cfd_tunnel

// Get tunnel
GET /accounts/{account_id}/cfd_tunnel/{tunnel_id}

// Update tunnel
PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}

// Delete tunnel
DELETE /accounts/{account_id}/cfd_tunnel/{tunnel_id}
```

### DNS Management

```go
// List DNS records
GET /zones/{zone_id}/dns_records

// Create DNS record
POST /zones/{zone_id}/dns_records

// Update DNS record
PUT /zones/{zone_id}/dns_records/{record_id}

// Delete DNS record
DELETE /zones/{zone_id}/dns_records/{record_id}
```

## Environment Variables

### Authentication

```bash
# Cloudflare API credentials
export CLOUDFLARE_API_KEY="your-api-key"
export CLOUDFLARE_EMAIL="your-email@example.com"

# Alternative: API Token
export CLOUDFLARE_API_TOKEN="your-api-token"
```

### Configuration

```bash
# Custom config directory
export CLOUDFLARED_CONFIG_DIR="/path/to/config"

# Tunnel credentials
export TUNNEL_TOKEN="your-tunnel-token"
```

## Common Use Cases

### 1. Simple HTTP Service

```bash
# Create and run tunnel for local web server
cloudflared tunnel create web-server
cloudflared tunnel route dns web-server example.com
cloudflared tunnel --url http://localhost:8080 run web-server
```

### 2. Multiple Services

```yaml
# config.yml
tunnel: <tunnel-id>
credentials-file: /path/to/credentials.json

ingress:
  - hostname: web.example.com
    service: http://localhost:8080
  - hostname: api.example.com
    service: http://localhost:3000
  - hostname: db.example.com
    service: tcp://localhost:5432
  - service: http_status:404
```

### 3. Development Environment

```bash
# Quick tunnel for development
cloudflared tunnel --url http://localhost:3000 --name dev-tunnel
```

## Error Handling

### Common Errors

1. **Authentication Issues**
   - Check cert.pem file exists
   - Verify domain is on Cloudflare
   - Ensure proper permissions

2. **DNS Resolution Issues**
   - Verify DNS records are created
   - Check propagation status
   - Confirm zone ID is correct

3. **Service Connection Issues**
   - Verify local service is running
   - Check firewall settings
   - Validate ingress rules

### Debug Steps

```bash
# 1. Check authentication
cloudflared tunnel login

# 2. Validate configuration
cloudflared tunnel ingress validate

# 3. Test ingress rules
cloudflared tunnel ingress rule https://your-domain.com

# 4. Check tunnel status
cloudflared tunnel list

# 5. View logs
cloudflared tunnel --loglevel debug run <tunnel-name>
```

## Security Best Practices

1. **API Keys**: Use API tokens instead of global API keys
2. **Certificates**: Protect cert.pem file with proper permissions
3. **Access Control**: Implement Cloudflare Access policies
4. **Monitoring**: Enable logging and monitoring
5. **Regular Updates**: Keep cloudflared updated

## Integration with Tunnelman

The Tunnelman TUI application uses these commands and APIs to:

- **List and manage tunnels** using `cloudflared tunnel list` and API calls
- **Create/delete tunnels** via CLI commands and API
- **Monitor tunnel status** through API polling
- **Manage DNS records** via Cloudflare API
- **Handle configuration** by reading/writing config files
- **Display logs** by parsing cloudflared output

For more information, see the [official Cloudflare Tunnel documentation](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/).