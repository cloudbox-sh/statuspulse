package client

import (
	"encoding/json"
	"time"
)

// Types mirror api/models.go. They're duplicated here (rather than imported)
// so the CLI module stays independent of the API module.

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Plan string `json:"plan"`
}

// PlanLimits mirrors api/plans.go — the `/api/auth/me` endpoint returns it
// alongside the user/orgs so the dashboard can soft-gate UI.
type PlanLimits struct {
	Plan            string `json:"Plan"`
	MaxMonitors     int    `json:"MaxMonitors"`
	MaxStatusPages  int    `json:"MaxStatusPages"`
	MinIntervalSecs int    `json:"MinIntervalSecs"`
	HistoryDays     int    `json:"HistoryDays"`
}

type Monitor struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	Target          string          `json:"target"`
	IntervalSeconds int             `json:"interval_seconds"`
	TimeoutSeconds  int             `json:"timeout_seconds"`
	Method          string          `json:"method"`
	ExpectedStatus  int             `json:"expected_status"`
	StatusRule      *string         `json:"status_rule"`
	Headers         json.RawMessage `json:"headers"`
	BodyAssertions  json.RawMessage `json:"body_assertions"`
	Enabled         bool            `json:"enabled"`
	CurrentStatus   string          `json:"current_status"`
	LastCheckedAt   *time.Time      `json:"last_checked_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type CheckResult struct {
	ID             string    `json:"id"`
	MonitorID      string    `json:"monitor_id"`
	Status         string    `json:"status"`
	ResponseTimeMs int       `json:"response_time_ms"`
	StatusCode     *int      `json:"status_code"`
	Error          *string   `json:"error"`
	Region         string    `json:"region"`
	CheckedAt      time.Time `json:"checked_at"`
}

type UptimeDaily struct {
	Date          string  `json:"date"`
	UptimePct     float64 `json:"uptime_pct"`
	AvgResponseMs float64 `json:"avg_response_ms"`
}

type UptimeReport struct {
	UptimePct float64       `json:"uptime_pct"`
	Days      int           `json:"days"`
	Daily     []UptimeDaily `json:"daily"`
}

type Incident struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Impact     string     `json:"impact"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

type IncidentUpdate struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incident_id"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}

// IncidentMonitor is the condensed monitor shape returned inside
// `GET /api/incidents/:id` — just enough to render a timeline.
type IncidentMonitor struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	CurrentStatus string `json:"current_status"`
}

type IncidentDetail struct {
	Incident Incident          `json:"incident"`
	Updates  []IncidentUpdate  `json:"updates"`
	Monitors []IncidentMonitor `json:"monitors"`
}

type StatusPage struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	OrgSlug      string   `json:"org_slug"`
	CustomDomain *string  `json:"custom_domain"`
	LogoURL      *string  `json:"logo_url"`
	HeaderText   string   `json:"header_text"`
	Theme        string   `json:"theme"`
	BrandColor   *string  `json:"brand_color"`
	SupportURL   *string  `json:"support_url"`
	Description  string   `json:"description"`
	FaviconURL   *string  `json:"favicon_url"`
	Enabled      bool     `json:"enabled"`
	MonitorIDs   []string `json:"monitor_ids"`
}

// StatusPageDetailMonitor is the per-monitor breakdown returned inside
// `GET /api/status-pages/:id` (with uptime % and daily history).
type StatusPageDetailMonitor struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	DisplayName   *string       `json:"display_name"`
	Type          string        `json:"type"`
	Target        string        `json:"target"`
	CurrentStatus string        `json:"current_status"`
	UptimePct     float64       `json:"uptime_pct"`
	Daily         []DailyUptime `json:"daily"`
	LastCheckedAt *time.Time    `json:"last_checked_at"`
}

type DailyUptime struct {
	Date      string  `json:"date"`
	UptimePct float64 `json:"uptime_pct"`
}

type StatusPageDetailIncident struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Status           string     `json:"status"`
	Impact           string     `json:"impact"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ResolvedAt       *time.Time `json:"resolved_at"`
	AffectedMonitors []string   `json:"affected_monitors"`
}

type StatusPageDetail struct {
	Page               StatusPage                 `json:"page"`
	Monitors           []StatusPageDetailMonitor  `json:"monitors"`
	AggregateDaily     []DailyUptime              `json:"aggregate_daily"`
	AggregateUptimePct float64                    `json:"aggregate_uptime_pct"`
	Incidents          []StatusPageDetailIncident `json:"incidents"`
	Days               int                        `json:"days"`
}

// StatusPageAttachment is the row returned by POST /status-pages/:id/monitors.
type StatusPageAttachment struct {
	ID           string  `json:"id"`
	StatusPageID string  `json:"status_page_id"`
	MonitorID    string  `json:"monitor_id"`
	DisplayName  *string `json:"display_name"`
	SortOrder    int     `json:"sort_order"`
}

// MaintenanceWindow mirrors api/models.go's MaintenanceWindow — a scheduled
// window during which the worker suppresses auto-incidents and notifications
// for monitors attached to the parent status page.
type MaintenanceWindow struct {
	ID           string    `json:"id"`
	StatusPageID string    `json:"status_page_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type NotificationChannel struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
}

type DashboardStats struct {
	MonitorsCount   int `json:"monitors_count"`
	UpCount         int `json:"up_count"`
	DownCount       int `json:"down_count"`
	IncidentsActive int `json:"incidents_active"`
}

// AuditEntry mirrors api/audit_handler.go's AuditEntry.
type AuditEntry struct {
	ID          string          `json:"id"`
	ActorUserID *string         `json:"actor_user_id"`
	ActorEmail  string          `json:"actor_email"`
	ActorName   string          `json:"actor_name"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entity_type"`
	EntityID    *string         `json:"entity_id"`
	EntityLabel string          `json:"entity_label"`
	Before      json.RawMessage `json:"before"`
	After       json.RawMessage `json:"after"`
	IPAddress   string          `json:"ip_address"`
	UserAgent   string          `json:"user_agent"`
	CreatedAt   time.Time       `json:"created_at"`
}

type AuditListResponse struct {
	Entries []AuditEntry `json:"entries"`
	Total   int          `json:"total"`
	Limit   int          `json:"limit"`
}

// PublicMonitor / PublicStatus mirror the shape returned by the unauthenticated
// GET /api/public/status/:slug endpoint.
type PublicMonitor struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	DisplayName *string `json:"display_name"`
	Uptime90d   float64 `json:"uptime_90d"`
}

type PublicStatus struct {
	Page      StatusPage      `json:"page"`
	Monitors  []PublicMonitor `json:"monitors"`
	Incidents []Incident      `json:"incidents"`
}
