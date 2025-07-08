# Docs
- Docs about the tunnel CLI tool is in docs/cloudflare.md

# Product Requirements Document

### CFTUI - Cloudflare Tunnel Management TUI

## 1. Product Overview

### 1.1 Purpose
CFTUI is a Terminal User Interface (TUI) application that provides developers with a fast, efficient way to manage Cloudflare tunnels and associated DNS records from the command line. It eliminates the need to switch between multiple tools and web interfaces when working with local development environments.

### 1.2 Target Users
- Full-stack developers running local development servers
- DevOps engineers managing staging environments
- Teams using Cloudflare tunnels for secure local development
- Developers who prefer terminal-based workflows

### 1.3 Key Benefits
- Single interface for tunnel and DNS management
- Faster workflow compared to web dashboard + CLI commands
- Visual status indicators for running services
- Persistent configuration management
- Cross-platform compatibility

---

## 2. Core Features

### 2.1 Tunnel Management
**Priority:** P0 (Must Have)

#### Requirements:
- List all configured tunnels with status indicators
- Start/stop tunnels with single keypress
- Create new tunnel configurations
- Delete existing tunnel configurations
- Real-time status updates (running/stopped/error)
- Display public URLs for active tunnels

#### Acceptance Criteria:
- Users can see all tunnels in a table format
- Green/red indicators clearly show tunnel status
- Tunnels start within 5 seconds of user action
- Error states are clearly communicated
- Public URLs are copyable/displayed prominently

### 2.2 DNS Record Management
**Priority:** P0 (Must Have)

#### Requirements:
- List DNS records for configured domain
- Create new DNS records pointing to local ports
- Delete existing DNS records
- Support A, CNAME, and AAAA record types
- Automatic
