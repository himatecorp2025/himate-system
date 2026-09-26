package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func (a *app) exportPartnersPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	lifecycle := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("lifecycle")))
	health := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("health")))
	referenceOnly, _ := strconv.ParseBool(r.URL.Query().Get("reference"))
	includeArchived, _ := strconv.ParseBool(r.URL.Query().Get("include_archived"))

	where := []string{"1=1"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		where = append(where, fmt.Sprintf(`(p.display_name ILIKE $%d OR p.legal_name ILIKE $%d OR p.id ILIKE $%d OR p.primary_domain ILIKE $%d)`, n, n, n, n))
	}
	if category != "" && category != "ALL" {
		add("p.category_id=$%d", category)
	}
	if lifecycle != "" && lifecycle != "ALL" {
		if !lifecycleValues[lifecycle] {
			common.APIError(w, 400, "VALIDATION", "Invalid lifecycle filter")
			return
		}
		add("p.lifecycle=$%d", lifecycle)
	} else if !includeArchived {
		where = append(where, "p.lifecycle<>'ARCHIVED'")
	}
	if health != "" && health != "ALL" {
		switch health {
		case "HEALTHY", "WARNING", "OFFLINE", "UNKNOWN":
			add("p.system_health=$%d", health)
		default:
			common.APIError(w, 400, "VALIDATION", "Invalid health filter")
			return
		}
	}
	if referenceOnly {
		where = append(where, "p.reference_partner=TRUE")
	}

	rows, err := a.db.QueryContext(r.Context(), `SELECT
		p.id,p.display_name,COALESCE(c.name_en,''),p.lifecycle,p.country,p.city,
		p.contact_email,p.system_health,p.platform_version,p.updated_at
		FROM partners.partners p
		LEFT JOIN partners.categories c ON c.id=p.category_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY p.reference_partner DESC,lower(p.display_name),p.id`, args...)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not export partners")
		return
	}
	defer rows.Close()

	tableRows := make([][]string, 0, 64)
	for rows.Next() {
		var id, displayName, categoryEN, lifecycle, country, city, contactEmail, health, platformVersion string
		var updatedAt time.Time
		if rows.Scan(&id, &displayName, &categoryEN, &lifecycle, &country, &city, &contactEmail, &health, &platformVersion, &updatedAt) != nil {
			continue
		}
		tableRows = append(tableRows, []string{
			displayName, id, categoryEN, lifecycle, country, city, contactEmail,
			health, platformVersion, updatedAt.UTC().Format("2006-01-02"),
		})
	}
	common.WriteBrandedTablePDF(
		w,
		"himate-partners.pdf",
		"HiMate Central - Partners",
		"Filtered partner portfolio export",
		[]string{"Partner", "ID", "Category", "Lifecycle", "Country", "City", "Contact", "Health", "Version", "Updated"},
		tableRows,
	)
}
