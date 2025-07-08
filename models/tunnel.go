package models

import (
	"time"
)

type TunnelStatus string

const (
	StatusActive   TunnelStatus = "active"
	StatusInactive TunnelStatus = "inactive"
	StatusError    TunnelStatus = "error"
	StatusUnknown  TunnelStatus = "unknown"
)

type TunnelConfig struct {
	URL      string            `json:"url"`
	Service  string            `json:"service"`
	Hostname string            `json:"hostname"`
	Headers  map[string]string `json:"headers,omitempty"`
	Path     string            `json:"path,omitempty"`
}

type Tunnel struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Status      TunnelStatus `json:"status"`
	Config      TunnelConfig `json:"config"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Connections []Connection `json:"connections,omitempty"`
}

type Connection struct {
	ID          string    `json:"id"`
	OriginIP    string    `json:"origin_ip"`
	Location    string    `json:"location"`
	Protocol    string    `json:"protocol"`
	ConnectedAt time.Time `json:"connected_at"`
	IsActive    bool      `json:"is_active"`
}

func NewTunnel(name, hostname, service string) *Tunnel {
	return &Tunnel{
		Name:   name,
		Status: StatusInactive,
		Config: TunnelConfig{
			Hostname: hostname,
			Service:  service,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (t *Tunnel) IsActive() bool {
	return t.Status == StatusActive
}

func (t *Tunnel) Update() {
	t.UpdatedAt = time.Now()
}
