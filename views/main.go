package views

import (
	"context"
	"fmt"
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
}

type tickMsg time.Time
type tunnelsLoadedMsg []models.CLITunnel
type dnsLoadedMsg []models.DNSRecord
type tunnelHostnamesLoadedMsg []models.PublicHostname
type errorMsg string
type statusMsg string

func NewModel(state *models.AppState, client *models.CloudflareClient, tunnelManager *models.TunnelManager) Model {
	return Model{
		state:         state,
		client:        client,
		tunnelManager: tunnelManager,
		tabs:          []string{"Tunnels"},
		activeTab:     0,
		statusMessage: "Ready",
		lastUpdate:    time.Now(),
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
			if len(m.tunnelsList) > 0 {
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

		case "escape":
			if m.showTunnelHostnames {
				m.showTunnelHostnames = false
				m.selectedTunnelName = ""
				m.selectedTunnelID = ""
				m.tunnelHostnames = nil
				m.statusMessage = "Returned to main view"
			}

		case "a":
			if m.showTunnelHostnames {
				m.showAddHostname = true
				m.initializeTextInputs()
				m.statusMessage = "Enter hostname for new public hostname"
			}

		case "e":
			if m.showTunnelHostnames && !m.showAddHostname && len(m.tunnelHostnames) > 0 {
				m.showEditHostname = true
				m.selectedHostname = m.tunnelHostnames[m.selectedHostnameIndex]
				m.initializeTextInputsForEdit()
				m.statusMessage = "Editing public hostname"
			}

		case "enter":
			if len(m.tunnelsList) > 0 && !m.showTunnelHostnames {
				cmds = append(cmds, m.toggleTunnel())
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

	case dnsLoadedMsg:
		m.dnsList = []models.DNSRecord(msg)
		m.loading = false
		m.statusMessage = fmt.Sprintf("Loaded %d DNS records", len(m.dnsList))

	case tunnelHostnamesLoadedMsg:
		m.tunnelHostnames = []models.PublicHostname(msg)
		m.loading = false
		m.statusMessage = fmt.Sprintf("Found %d public hostnames for tunnel: %s", len(m.tunnelHostnames), m.selectedTunnelName)

	case errorMsg:
		m.errorMessage = string(msg)
		m.loading = false
		m.statusMessage = "Error occurred"

	case statusMsg:
		m.statusMessage = string(msg)
		m.loading = false
		// If we just created or updated a public hostname, reload the tunnel hostnames
		if m.showTunnelHostnames && m.selectedTunnelName != "" && m.selectedTunnelID != "" {
			if (len(m.statusMessage) > len("Successfully created public hostname:") &&
				m.statusMessage[:len("Successfully created public hostname:")] == "Successfully created public hostname:") ||
				(len(m.statusMessage) > len("Successfully updated public hostname:") &&
				m.statusMessage[:len("Successfully updated public hostname:")] == "Successfully updated public hostname:") {
				cmds = append(cmds, m.loadTunnelHostnames(m.selectedTunnelID))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) initializeTextInputs() {
	m.textInputs = make([]textinput.Model, 3)

	// Hostname input
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

	// Hostname input
	m.textInputs[0] = textinput.New()
	m.textInputs[0].SetValue(m.selectedHostname.Hostname)
	m.textInputs[0].Focus()
	m.textInputs[0].CharLimit = 50
	m.textInputs[0].Width = 30

	// Path input
	m.textInputs[1] = textinput.New()
	m.textInputs[1].SetValue(m.selectedHostname.Path)
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

	case "esc":
		m.showAddHostname = false
		m.showEditHostname = false
		m.textInputs = nil
		m.focusIndex = 0
		m.statusMessage = "Cancelled"
		return m, nil

	case "tab", "shift+tab", "enter", "up", "down":
		s := msg.String()

		// Handle form navigation
		// Handle form navigation
		if s == "enter" && m.focusIndex == len(m.textInputs)-1 {
			// Submit form on last field
			hostnameInput := m.textInputs[0].Value()
			path := m.textInputs[1].Value()
			service := m.textInputs[2].Value()

			if hostnameInput == "" {
				m.statusMessage = "Hostname cannot be empty"
				return m, nil
			}

			// Construct full hostname with domain
			var fullHostname string
			if m.client != nil && m.client.GetZoneDomain() != "" {
				fullHostname = fmt.Sprintf("%s.%s", hostnameInput, m.client.GetZoneDomain())
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

		// Navigate between fields
		switch s {
		case "tab", "down", "enter":
			m.focusIndex++
			if m.focusIndex > len(m.textInputs)-1 {
				m.focusIndex = 0
			}
		case "shift+tab", "up":
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = len(m.textInputs) - 1
			}
		}

		m.updateFocus()
		return m, nil
	}

	// Update the focused text input
	m.textInputs[m.focusIndex], cmd = m.textInputs[m.focusIndex].Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) toggleTunnel() tea.Cmd {
	if m.selectedTunnel >= len(m.tunnelsList) {
		return nil
	}

	tunnel := m.tunnelsList[m.selectedTunnel]

	return tea.Cmd(func() tea.Msg {
		ctx := context.Background()
		status, err := m.client.GetTunnelStatus(ctx, tunnel.Name)
		if err != nil {
			return errorMsg(fmt.Sprintf("Failed to get tunnel status: %v", err))
		}

		if status == models.StatusActive {
			err = m.tunnelManager.StopTunnel(tunnel.Name)
			if err != nil {
				return errorMsg(fmt.Sprintf("Failed to stop tunnel: %v", err))
			}
			return statusMsg(fmt.Sprintf("Stopped tunnel: %s", tunnel.Name))
		} else {
			_, err = m.tunnelManager.StartTunnelWithURL(ctx, tunnel.Name, "http://localhost:8080")
			if err != nil {
				return errorMsg(fmt.Sprintf("Failed to start tunnel: %v", err))
			}
			return statusMsg(fmt.Sprintf("Started tunnel: %s", tunnel.Name))
		}
	})
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
		Foreground(lipgloss.Color("#6B7280")).
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
		Padding(1).
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
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true).
			Align(lipgloss.Center).
			Width(m.width - 8)
		return emptyStyle.Render("No tunnels found. Press 'r' to refresh or create a new tunnel.")
	}

	var rows []string

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#374151")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#D1D5DB")).
		PaddingBottom(1).
		MarginBottom(1)

	header := headerStyle.Render(fmt.Sprintf("%-20s %-15s %-10s %-30s", "NAME", "STATUS", "CONNECTIONS", "ID"))
	rows = append(rows, header)

	for i, tunnel := range m.tunnelsList {
		var style lipgloss.Style
		if i == m.selectedTunnel {
			style = lipgloss.NewStyle().
				Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)
		} else {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#374151"))
		}

		status := "Inactive"
		connections := fmt.Sprintf("%d", len(tunnel.Connections))

		if len(tunnel.Connections) > 0 {
			status = "Active"
		}

		statusColor := lipgloss.Color("#EF4444")
		if status == "Active" {
			statusColor = lipgloss.Color("#10B981")
		}

		statusStyled := lipgloss.NewStyle().
			Foreground(statusColor).
			Bold(true).
			Render(status)

		row := fmt.Sprintf("%-20s %-15s %-10s %-30s",
			tunnel.Name,
			statusStyled,
			connections,
			tunnel.ID[:8]+"...")

		rows = append(rows, style.Render(row))
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
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true).
			Align(lipgloss.Center).
			Width(m.width - 8)

		backInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			MarginTop(2).
			Italic(true).
			Align(lipgloss.Center).
			Width(m.width - 8).
			Render("Press 'a' to add, 'e' to edit • Spacebar/Escape to return to tunnels list")

		content := lipgloss.JoinVertical(lipgloss.Center,
			emptyStyle.Render("No public hostnames found for this tunnel"),
			backInfo,
		)

		return lipgloss.JoinVertical(lipgloss.Left, title, content)
	}

	var rows []string

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#374151")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#D1D5DB")).
		PaddingBottom(1).
		MarginBottom(1)

	header := headerStyle.Render(fmt.Sprintf("%-30s %-10s %-40s", "HOSTNAME", "PATH", "SERVICE"))
	rows = append(rows, header)

	for i, hostname := range m.tunnelHostnames {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#374151"))

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

		row := fmt.Sprintf("%-30s %-10s %-40s",
			hostname.Hostname,
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
		Render("Press 'a' to add, 'e' to edit • Spacebar/Escape to return to tunnels list")

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		backInfo,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

func (m Model) renderAddHostnameForm(title string) string {
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1).
		Width(m.width - 8).
		MarginTop(2)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#374151")).
		MarginBottom(1)

	// Field labels
	labels := []string{
		"Hostname (e.g., api):",
		"Path (defaults to *):",
		"Service (e.g., http://localhost:8080):",
	}

	var formContent []string

	// Render each text input with label
	for i, label := range labels {
		// Highlight current field label
		style := labelStyle
		if i == m.focusIndex {
			style = style.Foreground(lipgloss.Color("#7C3AED"))
		}

		formContent = append(formContent, style.Render(label))
		formContent = append(formContent, m.textInputs[i].View())

		// Add spacing between fields
		if i < len(labels)-1 {
			formContent = append(formContent, "")
		}
	}

	// Get the zone domain for hostname preview
	zoneDomain := "example.com"
	if m.client != nil {
		if domain := m.client.GetZoneDomain(); domain != "" {
			zoneDomain = domain
		}
	}

	// Add preview for hostname
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		MarginTop(1)

	var preview string
	hostname := m.textInputs[0].Value()
	service := m.textInputs[2].Value()
	if m.showEditHostname {
		if hostname == "" {
			preview = fmt.Sprintf("Will update: <hostname>.%s → %s", zoneDomain, service)
		} else {
			preview = fmt.Sprintf("Will update: %s.%s → %s", hostname, zoneDomain, service)
		}
	} else {
		if hostname == "" {
			preview = fmt.Sprintf("Will create: <hostname>.%s → %s", zoneDomain, service)
		} else {
			preview = fmt.Sprintf("Will create: %s.%s → %s", hostname, zoneDomain, service)
		}
	}

	previewText := previewStyle.Render(preview)
	formContent = append(formContent, "", previewText)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		MarginTop(2).
		Italic(true)

	help := helpStyle.Render("Tab: Next field • Enter: Submit • Escape: Cancel")
	formContent = append(formContent, help)

	content := lipgloss.JoinVertical(lipgloss.Left, formContent...)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		inputStyle.Render(content),
	)
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
		help = "Tab: Next field • Enter: Submit • Escape: Cancel • q: Quit"
	} else if m.showTunnelHostnames {
		help = "a: Add public hostname • e: Edit selected • Space/Escape: Back to tunnels • r: Refresh • c: Clear errors • h: Help • q: Quit"
	} else {
		help = "↑↓: Navigate • Enter: Start/Stop tunnel • Space: View public hostnames • d: Delete • r: Refresh • c: Clear errors • h: Help • q: Quit"
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
		Foreground(lipgloss.Color("#374151"))

	content := []string{
		titleStyle.Render("🔧 Tunnelman Help"),
		"",
		"NAVIGATION:",
		fmt.Sprintf("  %s      %s", keyStyle.Render("↑/↓ or k/j"), descStyle.Render("Navigate up/down in tunnel list")),
		"",
		"TUNNEL OPERATIONS:",
		fmt.Sprintf("  %s       %s", keyStyle.Render("Enter"), descStyle.Render("Start/Stop selected tunnel")),
		fmt.Sprintf("  %s       %s", keyStyle.Render("Space"), descStyle.Render("View public hostnames for tunnel")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("a"), descStyle.Render("Add public hostname (in tunnel hostname view)")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("d"), descStyle.Render("Delete selected tunnel")),
		"",
		"HOSTNAME OPERATIONS:",
		fmt.Sprintf("  %s           %s", keyStyle.Render("Tab"), descStyle.Render("Navigate between hostname form fields")),
		fmt.Sprintf("  %s       %s", keyStyle.Render("Enter"), descStyle.Render("Submit hostname form or move to next field")),
		"",
		"GENERAL:",
		fmt.Sprintf("  %s           %s", keyStyle.Render("r"), descStyle.Render("Refresh tunnels")),
		fmt.Sprintf("  %s           %s", keyStyle.Render("c"), descStyle.Render("Clear error messages")),
		fmt.Sprintf("  %s       %s", keyStyle.Render("h or ?"), descStyle.Render("Toggle this help")),
		fmt.Sprintf("  %s      %s", keyStyle.Render("q or Ctrl+C"), descStyle.Render("Quit application")),
		"",
		descStyle.Render("Press 'h' again to close this help screen."),
	}

	return helpStyle.Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}
