package models

import (
	"time"
)

type ViewMode string

const (
	ViewTunnels ViewMode = "tunnels"
	ViewDNS     ViewMode = "dns"
	ViewLogs    ViewMode = "logs"
	ViewHelp    ViewMode = "help"
)

type UIState struct {
	CurrentView    ViewMode `json:"current_view"`
	SelectedTunnel int      `json:"selected_tunnel"`
	SelectedDNS    int      `json:"selected_dns"`
	ShowHelp       bool     `json:"show_help"`
	FilterText     string   `json:"filter_text"`
	IsFiltering    bool     `json:"is_filtering"`
	WindowWidth    int      `json:"window_width"`
	WindowHeight   int      `json:"window_height"`
}

type AppState struct {
	Tunnels        []Tunnel    `json:"tunnels"`
	DNSRecords     []DNSRecord `json:"dns_records"`
	UI             UIState     `json:"ui"`
	LastSync       time.Time   `json:"last_sync"`
	ConfigPath     string      `json:"config_path"`
	SelectedDomain string      `json:"selected_domain"`
}

func NewAppState() *AppState {
	return &AppState{
		Tunnels:    make([]Tunnel, 0),
		DNSRecords: make([]DNSRecord, 0),
		UI: UIState{
			CurrentView:    ViewTunnels,
			SelectedTunnel: 0,
			SelectedDNS:    0,
			ShowHelp:       false,
			IsFiltering:    false,
		},
		LastSync: time.Now(),
	}
}

func (s *AppState) AddTunnel(tunnel *Tunnel) {
	s.Tunnels = append(s.Tunnels, *tunnel)
}

func (s *AppState) RemoveTunnel(id string) bool {
	for i, tunnel := range s.Tunnels {
		if tunnel.ID == id {
			s.Tunnels = append(s.Tunnels[:i], s.Tunnels[i+1:]...)
			return true
		}
	}
	return false
}

func (s *AppState) GetTunnel(id string) (*Tunnel, bool) {
	for i, tunnel := range s.Tunnels {
		if tunnel.ID == id {
			return &s.Tunnels[i], true
		}
	}
	return nil, false
}

func (s *AppState) AddDNSRecord(record *DNSRecord) {
	s.DNSRecords = append(s.DNSRecords, *record)
}

func (s *AppState) RemoveDNSRecord(id string) bool {
	for i, record := range s.DNSRecords {
		if record.ID == id {
			s.DNSRecords = append(s.DNSRecords[:i], s.DNSRecords[i+1:]...)
			return true
		}
	}
	return false
}

func (s *AppState) GetDNSRecord(id string) (*DNSRecord, bool) {
	for i, record := range s.DNSRecords {
		if record.ID == id {
			return &s.DNSRecords[i], true
		}
	}
	return nil, false
}

func (s *AppState) GetSelectedTunnel() *Tunnel {
	if s.UI.SelectedTunnel >= 0 && s.UI.SelectedTunnel < len(s.Tunnels) {
		return &s.Tunnels[s.UI.SelectedTunnel]
	}
	return nil
}

func (s *AppState) GetSelectedDNSRecord() *DNSRecord {
	if s.UI.SelectedDNS >= 0 && s.UI.SelectedDNS < len(s.DNSRecords) {
		return &s.DNSRecords[s.UI.SelectedDNS]
	}
	return nil
}

func (s *AppState) NextTunnel() {
	if len(s.Tunnels) > 0 {
		s.UI.SelectedTunnel = (s.UI.SelectedTunnel + 1) % len(s.Tunnels)
	}
}

func (s *AppState) PrevTunnel() {
	if len(s.Tunnels) > 0 {
		s.UI.SelectedTunnel = (s.UI.SelectedTunnel - 1 + len(s.Tunnels)) % len(s.Tunnels)
	}
}

func (s *AppState) NextDNSRecord() {
	if len(s.DNSRecords) > 0 {
		s.UI.SelectedDNS = (s.UI.SelectedDNS + 1) % len(s.DNSRecords)
	}
}

func (s *AppState) PrevDNSRecord() {
	if len(s.DNSRecords) > 0 {
		s.UI.SelectedDNS = (s.UI.SelectedDNS - 1 + len(s.DNSRecords)) % len(s.DNSRecords)
	}
}

func (s *AppState) SetView(view ViewMode) {
	s.UI.CurrentView = view
}

func (s *AppState) ToggleHelp() {
	s.UI.ShowHelp = !s.UI.ShowHelp
}

func (s *AppState) SetFilter(filter string) {
	s.UI.FilterText = filter
	s.UI.IsFiltering = filter != ""
}

func (s *AppState) UpdateWindowSize(width, height int) {
	s.UI.WindowWidth = width
	s.UI.WindowHeight = height
}

func (s *AppState) SetSelectedDomain(domain string) {
	s.SelectedDomain = domain
}

func (s *AppState) GetSelectedDomain() string {
	return s.SelectedDomain
}
