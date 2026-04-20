package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ── Auth ──

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}

func (c *Client) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	var out LoginResponse
	if err := c.do(ctx, "POST", "/api/auth/login", LoginRequest{Email: email, Password: password}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, "POST", "/api/auth/logout", nil, nil)
}

type MeResponse struct {
	User       User           `json:"user"`
	Orgs       []Organization `json:"orgs"`
	PlanLimits *PlanLimits    `json:"plan_limits"`
}

func (c *Client) Me(ctx context.Context) (*MeResponse, error) {
	var out MeResponse
	if err := c.do(ctx, "GET", "/api/auth/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Dashboard ──

func (c *Client) Dashboard(ctx context.Context) (*DashboardStats, error) {
	var out DashboardStats
	if err := c.do(ctx, "GET", "/api/dashboard", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Monitors ──

func (c *Client) ListMonitors(ctx context.Context) ([]Monitor, error) {
	var out []Monitor
	if err := c.do(ctx, "GET", "/api/monitors", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type monitorEnvelope struct {
	Monitor Monitor `json:"monitor"`
}

// MonitorDetail is the GET /api/monitors/:id response (monitor + recent
// checks + recent incidents touching this monitor).
type MonitorDetail struct {
	Monitor      Monitor       `json:"monitor"`
	RecentChecks []CheckResult `json:"recent_checks"`
	Incidents    []Incident    `json:"incidents"`
}

func (c *Client) GetMonitor(ctx context.Context, id string) (*MonitorDetail, error) {
	var out MonitorDetail
	if err := c.do(ctx, "GET", "/api/monitors/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type CreateMonitorRequest struct {
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	Target          string          `json:"target"`
	IntervalSeconds int             `json:"interval_seconds,omitempty"`
	TimeoutSeconds  int             `json:"timeout_seconds,omitempty"`
	Method          string          `json:"method,omitempty"`
	ExpectedStatus  int             `json:"expected_status,omitempty"`
	StatusRule      *string         `json:"status_rule,omitempty"`
	Headers         json.RawMessage `json:"headers,omitempty"`
	BodyAssertions  json.RawMessage `json:"body_assertions,omitempty"`
	Enabled         *bool           `json:"enabled,omitempty"`
}

func (c *Client) CreateMonitor(ctx context.Context, req CreateMonitorRequest) (*Monitor, error) {
	var out monitorEnvelope
	if err := c.do(ctx, "POST", "/api/monitors", req, &out); err != nil {
		return nil, err
	}
	return &out.Monitor, nil
}

// UpdateMonitorRequest mirrors the API's PATCH-like semantics: every field is
// a pointer, and a nil pointer means "leave unchanged". Empty strings are
// preserved (they're how the API clears nullable fields like status_rule).
type UpdateMonitorRequest struct {
	Name            *string          `json:"name,omitempty"`
	Target          *string          `json:"target,omitempty"`
	Type            *string          `json:"type,omitempty"`
	IntervalSeconds *int             `json:"interval_seconds,omitempty"`
	TimeoutSeconds  *int             `json:"timeout_seconds,omitempty"`
	Method          *string          `json:"method,omitempty"`
	ExpectedStatus  *int             `json:"expected_status,omitempty"`
	StatusRule      *string          `json:"status_rule,omitempty"`
	ClearStatusRule bool             `json:"clear_status_rule,omitempty"`
	Headers         *json.RawMessage `json:"headers,omitempty"`
	BodyAssertions  *json.RawMessage `json:"body_assertions,omitempty"`
	Enabled         *bool            `json:"enabled,omitempty"`
}

func (c *Client) UpdateMonitor(ctx context.Context, id string, req UpdateMonitorRequest) (*Monitor, error) {
	var out monitorEnvelope
	if err := c.do(ctx, "PUT", "/api/monitors/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out.Monitor, nil
}

func (c *Client) DeleteMonitor(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/api/monitors/"+id, nil, nil)
}

// ChecksQuery restricts results by time and row count. Zero Limit defaults
// server-side to 100.
type ChecksQuery struct {
	From  *time.Time
	To    *time.Time
	Limit int
}

func (q ChecksQuery) encode() string {
	v := url.Values{}
	if q.From != nil {
		v.Set("from", q.From.UTC().Format(time.RFC3339))
	}
	if q.To != nil {
		v.Set("to", q.To.UTC().Format(time.RFC3339))
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

func (c *Client) MonitorChecks(ctx context.Context, id string, q ChecksQuery) ([]CheckResult, error) {
	var out []CheckResult
	if err := c.do(ctx, "GET", "/api/monitors/"+id+"/checks"+q.encode(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MonitorUptime(ctx context.Context, id string, days int) (*UptimeReport, error) {
	path := "/api/monitors/" + id + "/uptime"
	if days > 0 {
		path += "?days=" + strconv.Itoa(days)
	}
	var out UptimeReport
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Incidents ──

func (c *Client) ListIncidents(ctx context.Context, statusFilter string) ([]Incident, error) {
	path := "/api/incidents"
	if statusFilter != "" {
		path += "?status=" + url.QueryEscape(statusFilter)
	}
	var out []Incident
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type CreateIncidentRequest struct {
	Title      string   `json:"title"`
	Status     string   `json:"status,omitempty"`
	Impact     string   `json:"impact,omitempty"`
	Message    string   `json:"message,omitempty"`
	MonitorIDs []string `json:"monitor_ids,omitempty"`
}

type incidentEnvelope struct {
	Incident Incident `json:"incident"`
}

func (c *Client) CreateIncident(ctx context.Context, req CreateIncidentRequest) (*Incident, error) {
	var out incidentEnvelope
	if err := c.do(ctx, "POST", "/api/incidents", req, &out); err != nil {
		return nil, err
	}
	return &out.Incident, nil
}

func (c *Client) GetIncident(ctx context.Context, id string) (*IncidentDetail, error) {
	var out IncidentDetail
	if err := c.do(ctx, "GET", "/api/incidents/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type UpdateIncidentRequest struct {
	Status *string `json:"status,omitempty"`
	Impact *string `json:"impact,omitempty"`
}

func (c *Client) UpdateIncident(ctx context.Context, id string, req UpdateIncidentRequest) (*Incident, error) {
	var out incidentEnvelope
	if err := c.do(ctx, "PUT", "/api/incidents/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out.Incident, nil
}

type AddIncidentUpdateRequest struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// AddIncidentUpdate posts a new update (timeline entry) on an incident.
// If Status is "resolved" the parent incident is marked resolved server-side.
func (c *Client) AddIncidentUpdate(ctx context.Context, id string, req AddIncidentUpdateRequest) (*IncidentUpdate, error) {
	var out struct {
		Update IncidentUpdate `json:"update"`
	}
	if err := c.do(ctx, "POST", "/api/incidents/"+id+"/updates", req, &out); err != nil {
		return nil, err
	}
	return &out.Update, nil
}

// ── Status Pages ──

func (c *Client) ListStatusPages(ctx context.Context) ([]StatusPage, error) {
	var out []StatusPage
	if err := c.do(ctx, "GET", "/api/status-pages", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type CreateStatusPageRequest struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	HeaderText  string   `json:"header_text,omitempty"`
	Theme       string   `json:"theme,omitempty"`
	Description string   `json:"description,omitempty"`
	BrandColor  *string  `json:"brand_color,omitempty"`
	SupportURL  *string  `json:"support_url,omitempty"`
	LogoURL     *string  `json:"logo_url,omitempty"`
	FaviconURL  *string  `json:"favicon_url,omitempty"`
	MonitorIDs  []string `json:"monitor_ids,omitempty"`
}

type statusPageEnvelope struct {
	StatusPage StatusPage `json:"status_page"`
}

func (c *Client) CreateStatusPage(ctx context.Context, req CreateStatusPageRequest) (*StatusPage, error) {
	var out statusPageEnvelope
	if err := c.do(ctx, "POST", "/api/status-pages", req, &out); err != nil {
		return nil, err
	}
	return &out.StatusPage, nil
}

func (c *Client) GetStatusPage(ctx context.Context, id string, days int) (*StatusPageDetail, error) {
	path := "/api/status-pages/" + id
	if days > 0 {
		path += "?days=" + strconv.Itoa(days)
	}
	var out StatusPageDetail
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateStatusPageRequest uses pointers + explicit empty-string semantics for
// nullable branding fields: a nil pointer leaves the field unchanged, while
// a pointer to "" clears it to NULL server-side.
type UpdateStatusPageRequest struct {
	Name         *string   `json:"name,omitempty"`
	Slug         *string   `json:"slug,omitempty"`
	CustomDomain *string   `json:"custom_domain,omitempty"`
	LogoURL      *string   `json:"logo_url,omitempty"`
	HeaderText   *string   `json:"header_text,omitempty"`
	Theme        *string   `json:"theme,omitempty"`
	Description  *string   `json:"description,omitempty"`
	BrandColor   *string   `json:"brand_color,omitempty"`
	SupportURL   *string   `json:"support_url,omitempty"`
	FaviconURL   *string   `json:"favicon_url,omitempty"`
	Enabled      *bool     `json:"enabled,omitempty"`
	MonitorIDs   *[]string `json:"monitor_ids,omitempty"`
}

func (c *Client) UpdateStatusPage(ctx context.Context, id string, req UpdateStatusPageRequest) (*StatusPage, error) {
	var out statusPageEnvelope
	if err := c.do(ctx, "PUT", "/api/status-pages/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out.StatusPage, nil
}

func (c *Client) DeleteStatusPage(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/api/status-pages/"+id, nil, nil)
}

type AttachMonitorRequest struct {
	MonitorID   string  `json:"monitor_id"`
	DisplayName *string `json:"display_name,omitempty"`
	SortOrder   *int    `json:"sort_order,omitempty"`
}

func (c *Client) AttachMonitor(ctx context.Context, pageID string, req AttachMonitorRequest) (*StatusPageAttachment, error) {
	var out struct {
		Attachment StatusPageAttachment `json:"status_page_monitor"`
	}
	if err := c.do(ctx, "POST", "/api/status-pages/"+pageID+"/monitors", req, &out); err != nil {
		return nil, err
	}
	return &out.Attachment, nil
}

func (c *Client) DetachMonitor(ctx context.Context, pageID, monitorID string) error {
	return c.do(ctx, "DELETE", "/api/status-pages/"+pageID+"/monitors/"+monitorID, nil, nil)
}

// ── Notifications ──

func (c *Client) ListNotifications(ctx context.Context) ([]NotificationChannel, error) {
	var out []NotificationChannel
	if err := c.do(ctx, "GET", "/api/notifications", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type CreateNotificationRequest struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type notificationEnvelope struct {
	Channel NotificationChannel `json:"channel"`
}

func (c *Client) CreateNotification(ctx context.Context, req CreateNotificationRequest) (*NotificationChannel, error) {
	var out notificationEnvelope
	if err := c.do(ctx, "POST", "/api/notifications", req, &out); err != nil {
		return nil, err
	}
	return &out.Channel, nil
}

type UpdateNotificationRequest struct {
	Name    *string          `json:"name,omitempty"`
	Config  *json.RawMessage `json:"config,omitempty"`
	Enabled *bool            `json:"enabled,omitempty"`
}

func (c *Client) UpdateNotification(ctx context.Context, id string, req UpdateNotificationRequest) (*NotificationChannel, error) {
	var out notificationEnvelope
	if err := c.do(ctx, "PUT", "/api/notifications/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out.Channel, nil
}

func (c *Client) DeleteNotification(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/api/notifications/"+id, nil, nil)
}

// ── Scheduled maintenance ──

type maintenanceEnvelope struct {
	Window MaintenanceWindow `json:"maintenance_window"`
}

// ListMaintenance returns every window for the status page, newest first.
func (c *Client) ListMaintenance(ctx context.Context, pageID string) ([]MaintenanceWindow, error) {
	var out []MaintenanceWindow
	if err := c.do(ctx, "GET", "/api/status-pages/"+pageID+"/maintenance", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScheduleMaintenanceRequest matches the API: title, optional description,
// and a starts_at / ends_at pair (RFC3339 UTC).
type ScheduleMaintenanceRequest struct {
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
}

func (c *Client) ScheduleMaintenance(ctx context.Context, pageID string, req ScheduleMaintenanceRequest) (*MaintenanceWindow, error) {
	var out maintenanceEnvelope
	if err := c.do(ctx, "POST", "/api/status-pages/"+pageID+"/maintenance", req, &out); err != nil {
		return nil, err
	}
	return &out.Window, nil
}

// UpdateMaintenanceRequest mirrors PATCH-like semantics: nil pointer = leave.
type UpdateMaintenanceRequest struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
}

func (c *Client) UpdateMaintenance(ctx context.Context, id string, req UpdateMaintenanceRequest) (*MaintenanceWindow, error) {
	var out maintenanceEnvelope
	if err := c.do(ctx, "PUT", "/api/maintenance/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out.Window, nil
}

// CancelMaintenance removes a scheduled window. The API returns 204 even if
// the window has already passed.
func (c *Client) CancelMaintenance(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/api/maintenance/"+id, nil, nil)
}

// ── Audit log ──

// AuditQuery encodes the filter params shared between list and export.
type AuditQuery struct {
	EntityType string
	Action     string
	Actor      string
	From       *time.Time
	To         *time.Time
	Q          string
	Offset     int
	Limit      int
}

func (q AuditQuery) encode() string {
	v := url.Values{}
	if q.EntityType != "" {
		v.Set("entity_type", q.EntityType)
	}
	if q.Action != "" {
		v.Set("action", q.Action)
	}
	if q.Actor != "" {
		v.Set("actor", q.Actor)
	}
	if q.From != nil {
		v.Set("from", q.From.UTC().Format(time.RFC3339))
	}
	if q.To != nil {
		v.Set("to", q.To.UTC().Format(time.RFC3339))
	}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Offset > 0 {
		v.Set("offset", strconv.Itoa(q.Offset))
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

func (c *Client) ListAudit(ctx context.Context, q AuditQuery) (*AuditListResponse, error) {
	var out AuditListResponse
	if err := c.do(ctx, "GET", "/api/audit"+q.encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExportAudit streams an audit export (CSV or JSON) into w. The caller decides
// where it ends up — stdout, a file, etc. `format` must be "csv" or "json".
func (c *Client) ExportAudit(ctx context.Context, format string, q AuditQuery, w io.Writer) error {
	path := "/api/audit/export." + format + q.encode()
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("X-API-Key", c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		var apiErr struct {
			Error string `json:"error"`
		}
		msg := ""
		if json.Unmarshal(body, &apiErr) == nil {
			msg = apiErr.Error
		}
		return &APIError{Status: resp.StatusCode, Message: msg}
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// ── Public status page (unauthenticated) ──

// PublicStatus fetches the public view of a status page. Pages are addressed
// by (orgSlug, pageSlug) since page slugs are only unique within an org.
func (c *Client) PublicStatus(ctx context.Context, orgSlug, pageSlug string) (*PublicStatus, error) {
	var out PublicStatus
	path := "/api/public/status/" + url.PathEscape(orgSlug) + "/" + url.PathEscape(pageSlug)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
