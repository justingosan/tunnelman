# E2E Testing Guide for Tunnelman

This document explains how to run and maintain the end-to-end (E2E) tests for Tunnelman.

## Overview

The E2E tests validate the complete functionality of Tunnelman by testing against real Cloudflare APIs. They create actual tunnels and hostnames, then clean them up automatically.

## Prerequisites

1. **Go 1.24+** installed
2. **cloudflared CLI** installed and available in PATH
3. **Cloudflare API credentials** configured
4. **Active Cloudflare account** with Zone access

## Setup

### 1. Configure Cloudflare Credentials

Ensure your `~/.tunnelman/config.json` contains valid credentials:

```json
{
  "cloudflare_api_key": "your-api-token-here",
  "cloudflare_zone_id": "your-zone-id-here",
  "auto_refresh_seconds": 30,
  "log_level": "info"
}
```

### 2. Install Dependencies

```bash
make deps
```

## Running Tests

### Quick Start

```bash
# Run all E2E tests
make e2e-test

# Run with verbose output
make e2e-test-verbose

# Run specific test
make e2e-test-specific TEST=TestE2E_TunnelLifecycle
```

### Manual Execution

```bash
# Direct script execution
./scripts/run_e2e_tests.sh

# With specific test pattern
./scripts/run_e2e_tests.sh TestE2E_HostnameManagement

# With custom timeout and verbosity
E2E_TIMEOUT=20m E2E_VERBOSE=2 ./scripts/run_e2e_tests.sh
```

### Go Test Commands

```bash
# Run all E2E tests
go test -v -timeout=10m ./e2e_test.go

# Run specific test function
go test -v -run=TestE2E_TunnelLifecycle ./e2e_test.go

# Run with race detection
go test -v -race -timeout=10m ./e2e_test.go
```

## Test Categories

### 1. Tunnel Lifecycle Tests (`TestE2E_TunnelLifecycle`)
- **Create Tunnel**: Tests tunnel creation
- **List Tunnels**: Verifies tunnel listing functionality
- **Get Tunnel Info**: Tests tunnel information retrieval
- **Get Tunnel Status**: Validates status checking

### 2. Hostname Management Tests (`TestE2E_HostnameManagement`)
- **Add Public Hostname**: Tests hostname creation
- **Update Public Hostname**: Tests hostname editing (the recently fixed feature)
- **List Public Hostnames**: Verifies hostname listing
- **Remove Public Hostname**: Tests hostname deletion

### 3. Tunnel Configuration Tests (`TestE2E_TunnelConfiguration`)
- **Get Tunnel Configuration**: Tests configuration retrieval
- **Update Tunnel Configuration**: Tests direct configuration updates

### 4. Error Handling Tests (`TestE2E_ErrorHandling`)
- **Invalid Tunnel Operations**: Tests error responses for non-existent tunnels
- **Invalid Hostname Operations**: Tests error handling for invalid hostname operations

### 5. Integration Workflow Tests (`TestE2E_Integration_Full_Workflow`)
- **Complete Tunnel Workflow**: Tests end-to-end scenarios combining all operations

## Test Data Management

### Naming Convention
All test resources use prefixes to avoid conflicts:
- Tunnels: `e2e-test-{name}-{timestamp}`
- Hostnames: `e2e-test-{name}-{timestamp}.{domain}`

### Automatic Cleanup
The test suite automatically cleans up all created resources:
- **Tunnels**: Deleted after each test suite
- **DNS Records**: Removed from your zone
- **Failed Cleanup**: Logged as warnings but doesn't fail tests

### Manual Cleanup
If tests are interrupted and cleanup fails:

```bash
# List tunnels with e2e-test prefix
cloudflared tunnel list | grep e2e-test

# Delete specific tunnel
cloudflared tunnel delete e2e-test-example-123456789

# List DNS records (replace ZONE_ID)
curl -X GET "https://api.cloudflare.com/client/v4/zones/ZONE_ID/dns_records" \
  -H "Authorization: Bearer YOUR_API_TOKEN" | jq '.result[] | select(.name | startswith("e2e-test"))'
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `E2E_TIMEOUT` | `10m` | Test timeout duration |
| `E2E_VERBOSE` | `1` | Verbosity level (1=normal, 2=verbose) |

## CI/CD Integration

### GitHub Actions
The repository includes a GitHub Actions workflow (`.github/workflows/e2e-tests.yml`) that:
- Runs tests on push/PR to main branches
- Runs daily scheduled tests
- Supports manual triggering with test pattern selection

### Required Secrets
Configure these secrets in your GitHub repository:
- `CLOUDFLARE_API_TOKEN`: Your Cloudflare API token
- `CLOUDFLARE_ZONE_ID`: Your Cloudflare zone ID

## Troubleshooting

### Common Issues

1. **"No Cloudflare API key configured"**
   - Check your `~/.tunnelman/config.json` file
   - Ensure the API key is not empty

2. **"Authentication failed"**
   - Verify your API token has the correct permissions
   - Check if the token is expired

3. **"Zone ID is required"**
   - Add your zone ID to the configuration
   - Ensure you have access to the zone

4. **"cloudflared not found"**
   - Install cloudflared CLI
   - Ensure it's in your PATH

5. **Tests timeout**
   - Increase timeout: `E2E_TIMEOUT=20m make e2e-test`
   - Check network connectivity to Cloudflare APIs

### Debug Mode

For detailed debugging:

```bash
# Enable verbose logging
E2E_VERBOSE=2 go test -v -run=TestE2E_TunnelLifecycle ./e2e_test.go

# Check what resources exist
cloudflared tunnel list
```

### Resource Limits

Be aware of Cloudflare limits:
- **Free Plan**: 1 tunnel per account
- **Rate Limits**: API calls per minute
- **DNS Records**: Limits per zone

## Best Practices

1. **Run tests in isolation**: Each test creates its own resources
2. **Monitor cleanup**: Check logs for cleanup warnings
3. **Use test zones**: Consider using a dedicated test zone
4. **Parallel execution**: Tests can run in parallel safely
5. **CI scheduling**: Run comprehensive tests nightly, quick tests on commits

## Contributing

When adding new E2E tests:

1. Follow the naming convention (`TestE2E_FeatureName`)
2. Use the test suite's helper methods for resource creation
3. Always add created resources to cleanup lists
4. Test both success and error scenarios
5. Add appropriate assertions and error messages

## Performance Considerations

- **Test Duration**: Full suite takes ~5-10 minutes
- **API Calls**: Each test makes multiple API calls
- **Rate Limiting**: Tests respect Cloudflare rate limits
- **Cleanup Time**: Resource cleanup adds ~30 seconds per test

For faster development iterations, run specific tests:
```bash
make e2e-test-specific TEST=TestE2E_HostnameManagement
```