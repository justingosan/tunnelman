package views

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"tunnelman/models"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	state                 *models.AppState
	client                *models.CloudflareClient
	tunnelManager         *models.TunnelManager
	activeTab             int
	tabs                  []string
	width                 int
	height                int
	statusMessage         string
	errorMessage          string
	showHelp              bool
	tunnelsList           []models.CLITunnel
	dnsList               []models.DNSRecord
	selectedTunnel        int
	loading               bool
	lastUpdate            time.Time
	showTunnelHostnames   bool
	tunnelHostnames       []models.PublicHostname
	selectedTunnelName    string
	selectedTunnelID      string
	showAddHostname       bool
	showEditHostname      bool
	selectedHostname      models.PublicHostname
	selectedHostnameIndex int
	textInputs            []textinput.Model
	focusIndex            int
	availableDomains      []string
	selectedDomainIndex   int
	tunnelDomainCounts    map[string]int
	tunnelStatuses        map[string]models.TunnelStatus
}

type tickMsg time.Time
type tunnelsLoadedMsg []models.CLITunnel
type dnsLoadedMsg []models.DNSRecord
type tunnelHostnamesLoadedMsg []models.PublicHostname
type domainsLoadedMsg []string
type tunnelDomainCountsLoadedMsg map[string]int
type tunnelStatusesLoadedMsg map[string]models.TunnelStatus
type errorMsg string
type statusMsg string

func NewModel(state *models.AppState, client *models.CloudflareClient, tunnelManager *models.TunnelManager) Model {
	return Model{
		state:              state,
		client:             client,
		tunnelManager:      tunnelManager,
		tabs:               []string{"Tunnels"},
		activeTab:          0,
		statusMessage:      "Ready",
		lastUpdate:         time.Now(),
		tunnelDomainCounts: make(map[string]int),
		tunnelStatuses:     make(map[string]models.TunnelStatus),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.loadTunnels(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) loadTunnels() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized - check your API credentials")
		}

		ctx := context.Background()
		tunnels, err := m.client.ListTunnels(ctx)
		if err != nil {
			if models.IsAuthenticationError(err) {
				return errorMsg("Authentication failed - check API key/token and permissions")
			}
			return errorMsg(fmt.Sprintf("Failed to load tunnels: %v", err))
		}
		return tunnelsLoadedMsg(tunnels)
	})
}

func (m Model) loadTunnelHostnames(tunnelID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized - check your API credentials")
		}

		ctx := context.Background()
		hostnames, err := m.client.GetPublicHostnames(ctx, tunnelID)
		if err != nil {
			if models.IsAuthenticationError(err) {
				return errorMsg("Authentication failed - check API key/token and permissions")
			}
			return errorMsg(fmt.Sprintf("Failed to load public hostnames: %v", err))
		}

		return tunnelHostnamesLoadedMsg(hostnames)
	})
}

func (m Model) loadDomains() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized - check your API credentials")
		}

		ctx := context.Background()
		domains, err := m.client.GetAvailableDomains(ctx)
		if err != nil {
			if models.IsAuthenticationError(err) {
				return errorMsg("Authentication failed - check API key/token and permissions")
			}
			return errorMsg(fmt.Sprintf("Failed to load domains: %v", err))
		}

		return domainsLoadedMsg(domains)
	})
}

func (m Model) loadTunnelDomainCounts() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized")
		}

		ctx := context.Background()
		domainCounts := make(map[string]int)

		// Count domains for each tunnel
		for _, tunnel := range m.tunnelsList {
			hostnames, err := m.client.GetPublicHostnames(ctx, tunnel.ID)
			if err != nil {
				// If we can't get hostnames, set count to 0 instead of failing
				domainCounts[tunnel.ID] = 0
				continue
			}
			domainCounts[tunnel.ID] = len(hostnames)
		}

		return tunnelDomainCountsLoadedMsg(domainCounts)
	})
}

func (m Model) updateSingleTunnelDomainCount(tunnelID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized")
		}

		ctx := context.Background()
		hostnames, err := m.client.GetPublicHostnames(ctx, tunnelID)
		if err != nil {
			// If we can't get hostnames, don't update the count
			return nil
		}

		// Create a map with just this tunnel's count
		domainCounts := make(map[string]int)
		domainCounts[tunnelID] = len(hostnames)

		return tunnelDomainCountsLoadedMsg(domainCounts)
	})
}

func (m Model) loadTunnelStatuses() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized")
		}

		ctx := context.Background()
		statuses := make(map[string]models.TunnelStatus)

		// Get status for each tunnel
		for _, tunnel := range m.tunnelsList {
			status, err := m.client.GetTunnelStatus(ctx, tunnel.ID)
			if err != nil {
				// If we can't get status, set to unknown instead of failing
				statuses[tunnel.ID] = models.StatusUnknown
				continue
			}
			statuses[tunnel.ID] = status
		}

		return tunnelStatusesLoadedMsg(statuses)
	})
}

func (m Model) createTunnelHostname(hostname, path, service string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized - check your API credentials")
		}

		ctx := context.Background()
		err := m.client.AddPublicHostname(ctx, m.selectedTunnelID, hostname, path, service)
		if err != nil {
			return errorMsg(fmt.Sprintf("Failed to create public hostname: %v", err))
		}

		return statusMsg(fmt.Sprintf("Successfully created public hostname: %s", hostname))
	})
}

func (m Model) updateTunnelHostname(hostname, path, service string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized - check your API credentials")
		}

		ctx := context.Background()
		err := m.client.UpdatePublicHostname(ctx, m.selectedTunnelID, m.selectedHostname.Hostname, hostname, path, service)
		if err != nil {
			return errorMsg(fmt.Sprintf("Failed to update public hostname: %v", err))
		}

		return statusMsg(fmt.Sprintf("Successfully updated public hostname: %s", hostname))
	})
}

func (m Model) deleteTunnelHostname() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not initialized - check your API credentials")
		}

		hostname := m.selectedHostname.Hostname
		path := m.selectedHostname.Path
		if path == "" {
			path = "*"
		}

		ctx := context.Background()
		err := m.client.RemovePublicHostname(ctx, m.selectedTunnelID, hostname, path)
		if err != nil {
			return errorMsg(fmt.Sprintf("Failed to delete public hostname: %v", err))
		}

		return statusMsg(fmt.Sprintf("Successfully deleted public hostname: %s", hostname))
	})
}

func (m Model) openTunnelInBrowser(tunnelID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.client == nil {
			return errorMsg("Cloudflare client not available")
		}

		accountID := m.client.GetAccountID()
		if accountID == "" {
			return errorMsg("Account ID not available")
		}

		url := fmt.Sprintf("https://one.dash.cloudflare.com/%s/networks/tunnels/cfd_tunnel/%s/edit?tab=publicHostname", accountID, tunnelID)

		// Use different commands based on the operating system
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			return errorMsg(fmt.Sprintf("Unsupported operating system: %s", runtime.GOOS))
		}

		if err := cmd.Start(); err != nil {
			return errorMsg(fmt.Sprintf("Failed to open browser: %v", err))
		}

		return statusMsg("Opened tunnel configuration in browser")
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.state.UpdateWindowSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		// Handle form input mode separately
		if m.showAddHostname || m.showEditHostname {
			return m.handleFormInput(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "h", "?":
			m.showHelp = !m.showHelp
			m.state.ToggleHelp()

		case "r":
			m.loading = true
			m.statusMessage = "Refreshing..."
			m.errorMessage = "" // Clear any previous errors
			cmds = append(cmds, m.loadTunnels())

		case "c":
			m.errorMessage = ""
			m.statusMessage = "Error cleared"

		case "up", "k":
			if m.showTunnelHostnames {
				if m.selectedHostnameIndex > 0 {
					m.selectedHostnameIndex--
				}
			} else if m.selectedTunnel > 0 {
				m.selectedTunnel--
			}

		case "down", "j":
			if m.showTunnelHostnames {
				if m.selectedHostnameIndex < len(m.tunnelHostnames)-1 {
					m.selectedHostnameIndex++
				}
			} else if m.selectedTunnel < len(m.tunnelsList)-1 {
				m.selectedTunnel++
			}

		case "d":
			if m.showTunnelHostnames && !m.showAddHostname && !m.showEditHostname && len(m.tunnelHostnames) > 0 {
				// Delete hostname when in hostname view
				m.selectedHostname = m.tunnelHostnames[m.selectedHostnameIndex]
				m.loading = true
				m.statusMessage = fmt.Sprintf("Deleting public hostname: %s", m.selectedHostname.Hostname)
				cmds = append(cmds, m.deleteTunnelHostname())
			} else if !m.showTunnelHostnames && len(m.tunnelsList) > 0 {
				// Delete tunnel when in main tunnel view
				cmds = append(cmds, m.deleteTunnel())
			}

		case " ":
			if m.showTunnelHostnames {
				// Exit tunnel hostnames view
				m.showTunnelHostnames = false
				m.selectedTunnelName = ""
				m.selectedTunnelID = ""
				m.tunnelHostnames = nil
				m.statusMessage = "Returned to main view"
			} else if m.activeTab == 0 && len(m.tunnelsList) > 0 {
				// Show public hostnames for selected tunnel
				tunnel := m.tunnelsList[m.selectedTunnel]
				m.selectedTunnelName = tunnel.Name
				m.selectedTunnelID = tunnel.ID
				m.showTunnelHostnames = true
				m.loading = true
				m.statusMessage = fmt.Sprintf("Loading public hostnames for tunnel: %s", tunnel.Name)
				cmds = append(cmds, m.loadTunnelHostnames(tunnel.ID))
			}

		case "esc", "escape":
			// Close help if open
			if m.showHelp {
				m.showHelp = false
				m.state.ToggleHelp()
				m.statusMessage = "Closed help"
			} else if m.showTunnelHostnames {
				// Return from hostname list to tunnel list
				m.showTunnelHostnames = false
				m.selectedTunnelName = ""
				m.selectedTunnelID = ""
				m.tunnelHostnames = nil
				m.statusMessage = "Returned to tunnel list"
			}
			// Note: Escape from main tunnel list does nothing (use 'q' to quit)

		case "a":
			if m.showTunnelHostnames {
				m.showAddHostname = true
				m.initializeTextInputs()
				m.statusMessage = "Enter hostname for new public hostname"
				// Load domains if not already loaded
				if len(m.availableDomains) == 0 {
					cmds = append(cmds, m.loadDomains())
				}
			}

		case "e":
			if m.showTunnelHostnames && !m.showAddHostname && len(m.tunnelHostnames) > 0 {
				m.showEditHostname = true
				m.selectedHostname = m.tunnelHostnames[m.selectedHostnameIndex]
				m.initializeTextInputsForEdit()
				m.statusMessage = "Editing public hostname"
				// Load domains if not already loaded
				if len(m.availableDomains) == 0 {
					cmds = append(cmds, m.loadDomains())
				}
			}

		case "enter":
			if len(m.tunnelsList) > 0 && !m.showTunnelHostnames {
				// Show public hostnames for selected tunnel
				tunnel := m.tunnelsList[m.selectedTunnel]
				m.selectedTunnelName = tunnel.Name
				m.selectedTunnelID = tunnel.ID
				m.showTunnelHostnames = true
				m.loading = true
				m.statusMessage = fmt.Sprintf("Loading public hostnames for tunnel: %s", tunnel.Name)
				cmds = append(cmds, m.loadTunnelHostnames(tunnel.ID))
			}

		case "o":
			if m.showTunnelHostnames && m.selectedTunnelID != "" {
				// Open tunnel configuration when in hostname view
				cmds = append(cmds, m.openTunnelInBrowser(m.selectedTunnelID))
			} else if len(m.tunnelsList) > 0 {
				// Open tunnel configuration when in tunnel list view
				tunnel := m.tunnelsList[m.selectedTunnel]
				cmds = append(cmds, m.openTunnelInBrowser(tunnel.ID))
			}

			// Remove tab navigation since we only have one tab now

			// Remove shift+tab navigation since we only have one tab now

		}

	case tickMsg:
		cmds = append(cmds, tickCmd(), m.loadTunnels())
		m.lastUpdate = time.Time(msg)

	case tunnelsLoadedMsg:
		m.tunnelsList = []models.CLITunnel(msg)
		m.loading = false
		m.statusMessage = fmt.Sprintf("Loaded %d tunnels", len(m.tunnelsList))
		// Load domain counts and statuses for each tunnel
		cmds = append(cmds, m.loadTunnelDomainCounts())
		cmds = append(cmds, m.loadTunnelStatuses())

	case dnsLoadedMsg:
		m.dnsList = []models.DNSRecord(msg)
		m.loading = false
		m.statusMessage = fmt.Sprintf("Loaded %d DNS records", len(m.dnsList))

	case tunnelHostnamesLoadedMsg:
		m.tunnelHostnames = []models.PublicHostname(msg)
		m.loading = false
		m.statusMessage = fmt.Sprintf("Found %d public hostnames for tunnel: %s", len(m.tunnelHostnames), m.selectedTunnelName)

	case domainsLoadedMsg:
		m.availableDomains = []string(msg)
		m.selectedDomainIndex = 0
		// Find the current zone domain if available
		currentDomain := m.client.GetZoneDomain()
		if currentDomain != "" {
			for i, domain := range m.availableDomains {
				if domain == currentDomain {
					m.selectedDomainIndex = i
					break
				}
			}
		}

	case tunnelDomainCountsLoadedMsg:
		// Merge the new counts with existing ones
		if m.tunnelDomainCounts == nil {
			m.tunnelDomainCounts = make(map[string]int)
		}
		for tunnelID, count := range msg {
			m.tunnelDomainCounts[tunnelID] = count
		}

	case tunnelStatusesLoadedMsg:
		// Merge the new statuses with existing ones
		if m.tunnelStatuses == nil {
			m.tunnelStatuses = make(map[string]models.TunnelStatus)
		}
		for tunnelID, status := range msg {
			m.tunnelStatuses[tunnelID] = status
		}

	case errorMsg:
		m.errorMessage = string(msg)
		m.loading = false
		m.statusMessage = "Error occurred"

	case statusMsg:
		m.statusMessage = string(msg)
		m.loading = false
		// If we just created or updated a public hostname, reload the tunnel hostnames and update domain count
		if m.showTunnelHostnames && m.selectedTunnelName != "" && m.selectedTunnelID != "" {
			if (len(m.statusMessage) > len("Successfully created public hostname:") &&
				m.statusMessage[:len("Successfully created public hostname:")] == "Successfully created public hostname:") ||
				(len(m.statusMessage) > len("Successfully updated public hostname:") &&
					m.statusMessage[:len("Successfully updated public hostname:")] == "Successfully updated public hostname:") ||
				(len(m.statusMessage) > len("Successfully deleted public hostname:") &&
					m.statusMessage[:len("Successfully deleted public hostname:")] == "Successfully deleted public hostname:") {
				cmds = append(cmds, m.loadTunnelHostnames(m.selectedTunnelID))
				// Also update the domain count for this tunnel
				cmds = append(cmds, m.updateSingleTunnelDomainCount(m.selectedTunnelID))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) initializeTextInputs() {
	m.textInputs = make([]textinput.Model, 3)

	// Hostname input (subdomain part only)
	m.textInputs[0] = textinput.New()
	m.textInputs[0].Placeholder = "api"
	m.textInputs[0].Focus()
	m.textInputs[0].CharLimit = 50
	m.textInputs[0].Width = 30

	// Path input
	m.textInputs[1] = textinput.New()
	m.textInputs[1].Placeholder = "*"
	m.textInputs[1].SetValue("*")
	m.textInputs[1].CharLimit = 20
	m.textInputs[1].Width = 20

	// Service input
	m.textInputs[2] = textinput.New()
	m.textInputs[2].Placeholder = "http://localhost:8080"
	m.textInputs[2].SetValue("http://localhost:8080")
	m.textInputs[2].CharLimit = 100
	m.textInputs[2].Width = 40

	m.focusIndex = 0
}

func (m *Model) initializeTextInputsForEdit() {
	m.textInputs = make([]textinput.Model, 3)

	// Parse hostname to separate subdomain and domain
	subdomain := ""
	if m.selectedHostname.Hostname != "" {
		parts := strings.SplitN(m.selectedHostname.Hostname, ".", 2)
		if len(parts) >= 1 {
			subdomain = parts[0]
		}
		if len(parts) == 2 {
			// Find the domain in our available domains list
			for i, domain := range m.availableDomains {
				if domain == parts[1] {
					m.selectedDomainIndex = i
					break
				}
			}
		}
	}

	// Hostname input (subdomain part only)
	m.textInputs[0] = textinput.New()
	m.textInputs[0].SetValue(subdomain)
	m.textInputs[0].Focus()
	m.textInputs[0].CharLimit = 50
	m.textInputs[0].Width = 30

	// Path input
	m.textInputs[1] = textinput.New()
	path := m.selectedHostname.Path
	if path == "" {
		path = "*"
	}
	m.textInputs[1].SetValue(path)
	m.textInputs[1].CharLimit = 20
	m.textInputs[1].Width = 20

	// Service input
	m.textInputs[2] = textinput.New()
	m.textInputs[2].SetValue(m.selectedHostname.Service)
	m.textInputs[2].CharLimit = 100
	m.textInputs[2].Width = 40

	m.focusIndex = 0
}

func (m *Model) updateFocus() {
	for i := 0; i < len(m.textInputs); i++ {
		if i == m.focusIndex {
			m.textInputs[i].Focus()
		} else {
			m.textInputs[i].Blur()
		}
	}
}

func (m Model) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle navigation and special keys
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc", "escape":
		// If in form, cancel form and return to hostname list
		if m.showAddHostname || m.showEditHostname {
			m.showAddHostname = false
			m.showEditHostname = false
			m.textInputs = nil
			m.focusIndex = 0
			m.statusMessage = "Cancelled"
			return m, nil
		}
		// If in hostname list, return to tunnel list
		if m.showTunnelHostnames {
			m.showTunnelHostnames = false
			m.selectedTunnelName = ""
			m.selectedTunnelID = ""
			m.tunnelHostnames = nil
			m.statusMessage = "Returned to tunnel list"
			return m, nil
		}

	case "tab", "shift+tab", "enter", "up", "down":
		s := msg.String()

		// Handle form navigation and domain selection
		if s == "enter" && m.focusIndex == 3 {
			// Submit form when focused on domain dropdown
			hostnameInput := m.textInputs[0].Value()
			path := m.textInputs[1].Value()
			service := m.textInputs[2].Value()

			if hostnameInput == "" {
				m.statusMessage = "Hostname cannot be empty"
				return m, nil
			}

			// Construct full hostname with selected domain
			var fullHostname string
			if len(m.availableDomains) > 0 && m.selectedDomainIndex < len(m.availableDomains) {
				fullHostname = fmt.Sprintf("%s.%s", hostnameInput, m.availableDomains[m.selectedDomainIndex])
			} else {
				fullHostname = hostnameInput // fallback if no domain available
			}

			m.loading = true
			if m.showEditHostname {
				m.showEditHostname = false
				m.statusMessage = fmt.Sprintf("Updating public hostname: %s", fullHostname)
				cmd = m.updateTunnelHostname(fullHostname, path, service)
			} else {
				m.showAddHostname = false
				m.statusMessage = fmt.Sprintf("Creating public hostname: %s", fullHostname)
				cmd = m.createTunnelHostname(fullHostname, path, service)
			}

			m.textInputs = nil
			m.focusIndex = 0
			return m, cmd
		}

		// Handle domain dropdown navigation when focused on it
		if m.focusIndex == 3 && len(m.availableDomains) > 0 {
			switch s {
			case "up":
				if m.selectedDomainIndex > 0 {
					m.selectedDomainIndex--
				}
				return m, nil
			case "down":
				if m.selectedDomainIndex < len(m.availableDomains)-1 {
					m.selectedDomainIndex++
				}
				return m, nil
			}
		}

		// Navigate between fields (including domain dropdown)
		// We have 3 text inputs (0,1,2) and 1 domain dropdown (3) = 4 total fields
		maxFocusIndex := 3 // domain dropdown is at index 3
		switch s {
		case "tab", "down", "enter":
			m.focusIndex++
			if m.focusIndex > maxFocusIndex {
				m.focusIndex = 0
			}
		case "shift+tab", "up":
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = maxFocusIndex
			}
		}

		m.updateFocus()
		return m, nil
	}

	// Update the focused text input (only if focus is on a text input, not domain dropdown)
	if m.focusIndex < len(m.textInputs) {
		m.textInputs[m.focusIndex], cmd = m.textInputs[m.focusIndex].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) deleteTunnel() tea.Cmd {
	if m.selectedTunnel >= len(m.tunnelsList) {
		return nil
	}

	tunnel := m.tunnelsList[m.selectedTunnel]

	return tea.Cmd(func() tea.Msg {
		ctx := context.Background()
		err := m.client.DeleteTunnel(ctx, tunnel.Name)
		if err != nil {
			return errorMsg(fmt.Sprintf("Failed to delete tunnel: %v", err))
		}
		return statusMsg(fmt.Sprintf("Deleted tunnel: %s", tunnel.Name))
	})
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content string

	if m.showHelp {
		content = m.renderHelp()
	} else {
		content = m.renderMainView()
	}

	return content
}

func (m Model) renderMainView() string {
	var sections []string

	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderTabs())
	sections = append(sections, m.renderContent())
	sections = append(sections, m.renderStatusBar())
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1).
		MarginBottom(1)

	title := titleStyle.Render("🌐 Tunnelman - Cloudflare Tunnel Manager")

	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Align(lipgloss.Right)

	timeStr := timeStyle.Width(m.width - lipgloss.Width(title)).
		Render(fmt.Sprintf("Last updated: %s", m.lastUpdate.Format("15:04:05")))

	return lipgloss.JoinHorizontal(lipgloss.Top, title, timeStr)
}

func (m Model) renderTabs() string {
	var tabs []string

	inactiveTabStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, false, true).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1).
		Foreground(lipgloss.Color("#9CA3AF"))

	activeTabStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, false, true).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Bold(true)

	for i, tab := range m.tabs {
		var style lipgloss.Style
		if i == m.activeTab {
			style = activeTabStyle
		} else {
			style = inactiveTabStyle
		}
		tabs = append(tabs, style.Render(tab))
	}

	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	gap := lipgloss.NewStyle().
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#374151")).
		Width(m.width - lipgloss.Width(tabsRow)).
		Render("")

	return lipgloss.JoinHorizontal(lipgloss.Bottom, tabsRow, gap)
}

func (m Model) renderContent() string {
	contentHeight := m.height - 8

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(2, 3).
		Height(contentHeight).
		Width(m.width - 4)

	var content string

	if m.loading {
		content = m.renderLoading()
	} else if m.showTunnelHostnames || m.showAddHostname || m.showEditHostname {
		content = m.renderTunnelHostnamesView()
	} else {
		content = m.renderTunnelsTab()
	}

	return contentStyle.Render(content)
}

func (m Model) renderLoading() string {
	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true).
		Align(lipgloss.Center).
		Width(m.width - 8)

	return loadingStyle.Render("⏳ Loading...")
}

func (m Model) renderTunnelsTab() string {
	if len(m.tunnelsList) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true).
			Align(lipgloss.Center).
			Width(m.width - 8)
		return emptyStyle.Render("No tunnels found. Press 'r' to refresh or create a new tunnel.")
	}

	// Define column widths
	nameWidth := 20
	statusWidth := 10
	domainsWidth := 8
	idWidth := 15

	// Header styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F3F4F6")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#D1D5DB")).
		PaddingBottom(1).
		MarginBottom(1)

	// Column header styles
	nameHeaderStyle := lipgloss.NewStyle().Width(nameWidth).Align(lipgloss.Left)
	statusHeaderStyle := lipgloss.NewStyle().Width(statusWidth).Align(lipgloss.Center)
	domainsHeaderStyle := lipgloss.NewStyle().Width(domainsWidth).Align(lipgloss.Center)
	idHeaderStyle := lipgloss.NewStyle().Width(idWidth).Align(lipgloss.Left)

	// Build header
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		nameHeaderStyle.Render("NAME"),
		statusHeaderStyle.Render("STATUS"),
		domainsHeaderStyle.Render("DOMAINS"),
		idHeaderStyle.Render("ID"),
	)

	var rows []string
	rows = append(rows, headerStyle.Render(headerRow))

	// Add spacing between header and data rows
	if len(m.tunnelsList) > 0 {
		rows = append(rows, "")
	}

	for i, tunnel := range m.tunnelsList {
		// Row styles
		var baseStyle lipgloss.Style
		if i == m.selectedTunnel {
			baseStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)
		} else {
			baseStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB"))
		}

		// Column styles for this row
		nameStyle := baseStyle.Copy().Width(nameWidth).Align(lipgloss.Left)
		statusStyle := baseStyle.Copy().Width(statusWidth).Align(lipgloss.Center)
		domainsStyle := baseStyle.Copy().Width(domainsWidth).Align(lipgloss.Center)
		idStyle := baseStyle.Copy().Width(idWidth).Align(lipgloss.Left)

		// Get tunnel status
		status := "DOWN"
		statusColor := lipgloss.Color("#EF4444") // Red for DOWN

		if tunnelStatus, exists := m.tunnelStatuses[tunnel.ID]; exists {
			switch tunnelStatus {
			case models.StatusActive:
				status = "HEALTHY"
				statusColor = lipgloss.Color("#10B981") // Green for HEALTHY
			case models.StatusInactive:
				status = "DOWN"
				statusColor = lipgloss.Color("#EF4444") // Red for DOWN
			case models.StatusError:
				status = "ERROR"
				statusColor = lipgloss.Color("#F59E0B") // Yellow for ERROR
			case models.StatusUnknown:
				status = "UNKNOWN"
				statusColor = lipgloss.Color("#6B7280") // Gray for UNKNOWN
			}
		}

		// Get domain count for this tunnel
		domainCount := 0
		if count, exists := m.tunnelDomainCounts[tunnel.ID]; exists {
			domainCount = count
		}

		// Truncate long tunnel names
		tunnelName := tunnel.Name
		if len(tunnelName) > nameWidth-2 {
			tunnelName = tunnelName[:nameWidth-5] + "..."
		}

		statusText := status
		if i != m.selectedTunnel {
			// Apply color only when not selected (to avoid conflicts with selection highlight)
			statusText = lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(status)
		}

		// Short ID
		shortID := tunnel.ID
		if len(shortID) > idWidth-1 {
			shortID = shortID[:idWidth-4] + "..."
		}

		// Build row
		row := lipgloss.JoinHorizontal(lipgloss.Top,
			nameStyle.Render(tunnelName),
			statusStyle.Render(statusText),
			domainsStyle.Render(fmt.Sprintf("%d", domainCount)),
			idStyle.Render(shortID),
		)

		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderTunnelHostnamesView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#7C3AED")).
		PaddingBottom(1).
		MarginBottom(2)

	title := titleStyle.Render(fmt.Sprintf("🌐 Public Hostnames for Tunnel: %s", m.selectedTunnelName))

	// Show hostname input form if adding a new hostname
	if m.showAddHostname || m.showEditHostname {
		return m.renderAddHostnameForm(title)
	}

	if len(m.tunnelHostnames) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true).
			Align(lipgloss.Center).
			Width(m.width - 8)

		backInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			MarginTop(2).
			Italic(true).
			Align(lipgloss.Center).
			Width(m.width - 8).
			Render("Press 'a' to add, 'e' to edit, 'd' to delete, 'o' to open in browser • Spacebar/Escape to return to tunnels list")

		content := lipgloss.JoinVertical(lipgloss.Center,
			emptyStyle.Render("No public hostnames found for this tunnel"),
			backInfo,
		)

		return lipgloss.JoinVertical(lipgloss.Left, title, content)
	}

	var rows []string

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F3F4F6")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#D1D5DB")).
		PaddingBottom(1).
		MarginBottom(1)

	header := headerStyle.Render(fmt.Sprintf("%-30s %-10s %-40s", "HOSTNAME", "PATH", "SERVICE"))
	rows = append(rows, header)

	for i, hostname := range m.tunnelHostnames {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

		if i == m.selectedHostnameIndex {
			style = style.Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)
		}

		path := hostname.Path
		if path == "" {
			path = "*"
		}

		service := hostname.Service
		if len(service) > 40 {
			service = service[:37] + "..."
		}

		// Add https:// prefix to hostname for display since all Cloudflare tunnels are HTTPS
		displayHostname := "https://" + hostname.Hostname
		if len(displayHostname) > 30 {
			displayHostname = displayHostname[:27] + "..."
		}

		row := fmt.Sprintf("%-30s %-10s %-40s",
			displayHostname,
			path,
			service)

		rows = append(rows, style.Render(row))
	}

	backInfo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		MarginTop(2).
		Italic(true).
		Align(lipgloss.Center).
		Width(m.width - 8).
		Render("Press 'a' to add, 'e' to edit, 'd' to delete, 'o' to open in browser • Spacebar/Escape to return to tunnels list")

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		backInfo,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

func (m Model) renderAddHostnameForm(title string) string {
	// No additional styling here since renderContent() already provides borders and padding
	formStyle := lipgloss.NewStyle().
		MarginTop(1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#D1D5DB"))

	focusedLabelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	var formContent []string

	// Ensure we have text inputs initialized
	if len(m.textInputs) < 3 {
		formContent = append(formContent, "Loading form...")
		return lipgloss.JoinVertical(lipgloss.Left, title, formStyle.Render(lipgloss.JoinVertical(lipgloss.Left, formContent...)))
	}

	// Hostname field
	style := labelStyle
	if m.focusIndex == 0 {
		style = focusedLabelStyle
	}
	formContent = append(formContent, style.Render("Hostname (e.g., api):"))
	formContent = append(formContent, m.textInputs[0].View())
	formContent = append(formContent, "")

	// Path field
	style = labelStyle
	if m.focusIndex == 1 {
		style = focusedLabelStyle
	}
	formContent = append(formContent, style.Render("Path (defaults to *):"))
	formContent = append(formContent, m.textInputs[1].View())
	formContent = append(formContent, "")

	// Service field
	style = labelStyle
	if m.focusIndex == 2 {
		style = focusedLabelStyle
	}
	formContent = append(formContent, style.Render("Service (e.g., http://localhost:8080):"))
	formContent = append(formContent, m.textInputs[2].View())
	formContent = append(formContent, "")

	// Domain dropdown field
	style = labelStyle
	if m.focusIndex == 3 {
		style = focusedLabelStyle
	}
	formContent = append(formContent, style.Render("Domain:"))
	formContent = append(formContent, m.renderDomainDropdown(m.focusIndex == 3))

	// Add preview for hostname
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true).
		MarginTop(1)

	var preview string
	hostname := ""
	service := ""

	// Safely get values from text inputs
	if len(m.textInputs) > 0 {
		hostname = m.textInputs[0].Value()
	}
	if len(m.textInputs) > 2 {
		service = m.textInputs[2].Value()
	}

	// Get selected domain
	selectedDomain := "example.com"
	if len(m.availableDomains) > 0 && m.selectedDomainIndex >= 0 && m.selectedDomainIndex < len(m.availableDomains) {
		selectedDomain = m.availableDomains[m.selectedDomainIndex]
	}

	if m.showEditHostname {
		if hostname == "" {
			preview = fmt.Sprintf("Will update: <hostname>.%s → %s", selectedDomain, service)
		} else {
			preview = fmt.Sprintf("Will update: %s.%s → %s", hostname, selectedDomain, service)
		}
	} else {
		if hostname == "" {
			preview = fmt.Sprintf("Will create: <hostname>.%s → %s", selectedDomain, service)
		} else {
			preview = fmt.Sprintf("Will create: %s.%s → %s", hostname, selectedDomain, service)
		}
	}

	previewText := previewStyle.Render(preview)
	formContent = append(formContent, "", previewText)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		MarginTop(2).
		Italic(true)

	help := helpStyle.Render("Tab: Next field • Up/Down: Select domain • Enter: Submit • Escape: Cancel")
	formContent = append(formContent, help)

	content := lipgloss.JoinVertical(lipgloss.Left, formContent...)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		formStyle.Render(content),
	)
}

func (m Model) renderDomainDropdown(focused bool) string {
	if len(m.availableDomains) == 0 {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true)
		return loadingStyle.Render("Loading domains...")
	}

	var style lipgloss.Style
	if focused {
		style = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(0, 1).
			Background(lipgloss.Color("#F3F4F6"))
	} else {
		style = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#D1D5DB")).
			Padding(0, 1)
	}

	selectedDomain := "example.com"
	if len(m.availableDomains) > 0 && m.selectedDomainIndex >= 0 && m.selectedDomainIndex < len(m.availableDomains) {
		selectedDomain = m.availableDomains[m.selectedDomainIndex]
	}

	content := selectedDomain
	if focused && len(m.availableDomains) > 1 {
		content += " ▼"
	}

	return style.Width(30).Render(content)
}

func (m Model) renderStatusBar() string {
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#374151")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Width(m.width)

	status := m.statusMessage
	if m.errorMessage != "" {
		errorStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#EF4444")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		status = errorStyle.Render("ERROR: " + m.errorMessage)
	}

	return statusStyle.Render(status)
}

func (m Model) renderFooter() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		Align(lipgloss.Center).
		Width(m.width)

	var help string
	if m.showAddHostname || m.showEditHostname {
		help = "Tab: Next field • Up/Down: Select domain • Enter: Submit • Escape: Cancel • q: Quit"
	} else if m.showTunnelHostnames {
		help = "a: Add hostname • e: Edit selected • d: Delete selected • Escape: Back to tunnels • r: Refresh • c: Clear errors • h: Help • q: Quit"
	} else {
		help = "↑↓: Navigate • Enter/Space: View hostnames • d: Delete tunnel • Escape: Close help • r: Refresh • h: Help • q: Quit"
	}

	return helpStyle.Render(help)
}

func (m Model) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(2).
		Width(m.width - 4).
		Height(m.height - 4)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		Align(lipgloss.Center).
		MarginBottom(2)

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#10B981"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D1D5DB"))

	content := []string{
		titleStyle.Render("🔧 Tunnelman Help"),
		"",
		"NAVIGATION:",
		fmt.Sprintf("  %s      %s", keyStyle.Render("↑/↓ or k/j"), descStyle.Render("Navigate up/down in tunnel list")),
		"",
		"TUNNEL OPERATIONS:",
		fmt.Sprintf("  %s       %s", keyStyle.Render("Enter"), descStyle.Render("View public hostnames for tunnel")),
		fmt.Sprintf("  %s       %s", keyStyle.Render("Space"), descStyle.Render("View public hostnames for tunnel")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("o"), descStyle.Render("Open tunnel configuration in browser (works in both tunnel and hostname views)")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("a"), descStyle.Render("Add public hostname (in tunnel hostname view)")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("e"), descStyle.Render("Edit public hostname (in tunnel hostname view)")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("d"), descStyle.Render("Delete selected tunnel or hostname (context-sensitive)")),
		"",
		"HOSTNAME OPERATIONS:",
		fmt.Sprintf("  %s           %s", keyStyle.Render("Tab"), descStyle.Render("Navigate between hostname form fields")),
		fmt.Sprintf("  %s       %s", keyStyle.Render("Enter"), descStyle.Render("Submit hostname form or move to next field")),
		"",
		"GENERAL:",
		fmt.Sprintf("  %s           %s", keyStyle.Render("r"), descStyle.Render("Refresh tunnels")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("c"), descStyle.Render("Clear error messages")),
		fmt.Sprintf("  %s       %s", keyStyle.Render("h or ?"), descStyle.Render("Toggle this help")),
		fmt.Sprintf("  %s         %s", keyStyle.Render("Escape"), descStyle.Render("Go back/cancel (close help, exit forms, return to tunnel list)")),
		fmt.Sprintf("  %s      %s", keyStyle.Render("q or Ctrl+C"), descStyle.Render("Quit application")),
		"",
		descStyle.Render("Press 'h' again to close this help screen."),
	}

	return helpStyle.Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}
