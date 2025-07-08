package models

import (
	"time"
)

type DNSRecordType string

const (
	RecordTypeA     DNSRecordType = "A"
	RecordTypeAAAA  DNSRecordType = "AAAA"
	RecordTypeCNAME DNSRecordType = "CNAME"
	RecordTypeMX    DNSRecordType = "MX"
	RecordTypeTXT   DNSRecordType = "TXT"
	RecordTypeSRV   DNSRecordType = "SRV"
	RecordTypeNS    DNSRecordType = "NS"
	RecordTypePTR   DNSRecordType = "PTR"
)

type DNSRecord struct {
	ID       string        `json:"id"`
	ZoneID   string        `json:"zone_id"`
	Name     string        `json:"name"`
	Type     DNSRecordType `json:"type"`
	Content  string        `json:"content"`
	Proxied  bool          `json:"proxied"`
	TTL      int           `json:"ttl"`
	Priority int           `json:"priority,omitempty"`
	Comment  string        `json:"comment,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func NewDNSRecord(name, recordType, content string, proxied bool) *DNSRecord {
	return &DNSRecord{
		Name:      name,
		Type:      DNSRecordType(recordType),
		Content:   content,
		Proxied:   proxied,
		TTL:       1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (d *DNSRecord) IsProxied() bool {
	return d.Proxied
}

func (d *DNSRecord) Update() {
	d.UpdatedAt = time.Now()
}

func (d *DNSRecord) CanBeProxied() bool {
	return d.Type == RecordTypeA || d.Type == RecordTypeAAAA || d.Type == RecordTypeCNAME
}