package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/client"
	"github.com/cloudbox-sh/statuspulse/internal/styles"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Inspect the SOC 2 / GDPR-aligned append-only audit log",
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditListCmd)
	auditCmd.AddCommand(auditExportCmd)

	initAuditListFlags()
	initAuditExportFlags()
}

// Shared filter flags (used by both list and export).
var (
	auditEntityType string
	auditAction     string
	auditActor      string
	auditFrom       string
	auditTo         string
	auditQ          string
)

func bindAuditFilterFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&auditEntityType, "entity-type", "",
		"Filter by entity: monitor, status_page, incident, incident_update, notification_channel")
	f.StringVar(&auditAction, "action", "", "Filter by action: created, updated, deleted")
	f.StringVar(&auditActor, "actor", "", "Filter by actor user UUID")
	f.StringVar(&auditFrom, "from", "", "Window start, RFC3339")
	f.StringVar(&auditTo, "to", "", "Window end, RFC3339")
	f.StringVar(&auditQ, "q", "", "Text search across entity label + actor email")
}

func buildAuditQuery() (client.AuditQuery, error) {
	q := client.AuditQuery{
		EntityType: auditEntityType,
		Action:     auditAction,
		Actor:      auditActor,
		Q:          auditQ,
	}
	if auditFrom != "" {
		t, err := time.Parse(time.RFC3339, auditFrom)
		if err != nil {
			return q, fmt.Errorf("--from: %w", err)
		}
		q.From = &t
	}
	if auditTo != "" {
		t, err := time.Parse(time.RFC3339, auditTo)
		if err != nil {
			return q, fmt.Errorf("--to: %w", err)
		}
		q.To = &t
	}
	return q, nil
}

// ── list ──────────────────────────────────────────────────────────────

var (
	auditListLimit  int
	auditListOffset int
)

func initAuditListFlags() {
	bindAuditFilterFlags(auditListCmd)
	auditListCmd.Flags().IntVar(&auditListLimit, "limit", 100, "Rows per page (1-500)")
	auditListCmd.Flags().IntVar(&auditListOffset, "offset", 0, "Rows to skip (pagination)")
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "Browse audit entries with filters",
	RunE:  runAuditList,
}

func runAuditList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	q, err := buildAuditQuery()
	if err != nil {
		return err
	}
	q.Limit = auditListLimit
	q.Offset = auditListOffset

	resp, err := c.ListAudit(ctx, q)
	if err != nil {
		return handleAPIError(err)
	}
	if jsonOutput {
		return emit(resp, nil)
	}
	if len(resp.Entries) == 0 {
		fmt.Println(styles.Dim.Render("no audit entries match those filters"))
		return nil
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Header.Width(20).Render("WHEN"),
		styles.Header.Width(10).Render("ACTION"),
		styles.Header.Width(22).Render("ENTITY"),
		styles.Header.Width(28).Render("LABEL"),
		styles.Header.Width(28).Render("ACTOR"),
		styles.Header.Width(16).Render("IP"),
	)
	fmt.Println(header)
	for _, e := range resp.Entries {
		actor := e.ActorEmail
		if actor == "" {
			actor = e.ActorName
		}
		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Cell.Width(20).Render(e.CreatedAt.Local().Format("2006-01-02 15:04:05")),
			styles.Cell.Width(10).Render(colorAuditAction(e.Action)),
			styles.Cell.Width(22).Render(e.EntityType),
			styles.Cell.Width(28).Render(truncate(e.EntityLabel, 26)),
			styles.Cell.Width(28).Render(truncate(actor, 26)),
			styles.Cell.Width(16).Render(truncate(e.IPAddress, 14)),
		))
	}
	fmt.Println()
	fmt.Println(styles.Faint.Render(fmt.Sprintf(
		"showing %d of %d total (offset %d, limit %d)",
		len(resp.Entries), resp.Total, auditListOffset, resp.Limit)))
	return nil
}

// ── export ────────────────────────────────────────────────────────────

var (
	auditExportFormat string
	auditExportOut    string
)

func initAuditExportFlags() {
	bindAuditFilterFlags(auditExportCmd)
	auditExportCmd.Flags().StringVar(&auditExportFormat, "format", "csv",
		"Export format: csv or json")
	auditExportCmd.Flags().StringVarP(&auditExportOut, "output", "o", "",
		"Write to file instead of stdout")
}

var auditExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export audit entries (CSV or JSON) — same filters as list",
	RunE:  runAuditExport,
}

func runAuditExport(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	switch auditExportFormat {
	case "csv", "json":
	default:
		return fmt.Errorf("--format must be csv or json")
	}

	q, err := buildAuditQuery()
	if err != nil {
		return err
	}

	out := os.Stdout
	if auditExportOut != "" {
		f, err := os.Create(auditExportOut)
		if err != nil {
			return fmt.Errorf("open output: %w", err)
		}
		defer f.Close()
		out = f
	}
	if err := c.ExportAudit(ctx, auditExportFormat, q, out); err != nil {
		return handleAPIError(err)
	}
	if auditExportOut != "" {
		fmt.Fprintln(os.Stderr, styles.Check()+" exported to "+auditExportOut)
	}
	return nil
}

func colorAuditAction(a string) string {
	switch a {
	case "created":
		return styles.Success.Render(a)
	case "deleted":
		return styles.Error.Render(a)
	case "updated":
		return styles.Warning.Render(a)
	}
	return styles.Dim.Render(a)
}
