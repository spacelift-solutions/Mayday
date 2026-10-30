package model

import "time"

type License struct {
	Service string `json:"service"`
	Key     string `json:"license_key"`
	Enabled bool   `json:"enabled"`
}

type Health struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	License   string    `json:"license"`
	Detail    string    `json:"detail,omitempty"`
	Bucket    string    `json:"bucket,omitempty"`
	Key       string    `json:"key,omitempty"`
	Workers   int       `json:"workers"`
	Active    int       `json:"active"`
	Queued    int       `json:"queued"`
	Completed uint64    `json:"completed"`
	Rejected  uint64    `json:"rejected"`
	CheckedAt time.Time `json:"checked_at"`
}

type Observation struct {
	At          time.Time `json:"at"`
	Reachable   bool      `json:"reachable"`
	Health      Health    `json:"health"`
	RequestRate int       `json:"request_rate"`
	Samples     uint64    `json:"samples"`
	Errors      uint64    `json:"errors"`
	ErrorRate   float64   `json:"error_rate"`
	LatencyMS   float64   `json:"latency_ms"`
}

type Incident struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Status   string     `json:"status"`
	OpenedAt time.Time  `json:"opened_at"`
	ClosedAt *time.Time `json:"closed_at,omitempty"`
	Message  string     `json:"message"`
}

type Event struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Service     string      `json:"service"`
	At          time.Time   `json:"at"`
	Incident    *Incident   `json:"incident,omitempty"`
	Observation Observation `json:"observation"`
	StackID     string      `json:"stack_id,omitempty"`
}
