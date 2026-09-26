package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func (a *app) exportPartnersCSV(w http.ResponseWriter, r *http.Request) {
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
		p.id,p.display_name,p.legal_name,COALESCE(c.name_en,''),COALESCE(c.name_hu,''),
		p.lifecycle,p.existing_partner,p.reference_partner,p.test_partner,
		p.primary_domain,p.country,p.state_region,p.city,p.contact_name,p.contact_email,
		p.finance_contact_name,p.finance_contact_email,p.system_health,p.platform_version,
		p.created_at,p.updated_at
		FROM partners.partners p
		LEFT JOIN partners.categories c ON c.id=p.category_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY p.reference_partner DESC,lower(p.display_name),p.id`, args...)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not export partners")
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="himate-partners.csv"`)
	w.Header().Set("Cache-Control", "private, no-store")
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{
		"partner_id", "display_name", "legal_name", "category_en", "category_hu",
		"lifecycle", "existing_partner", "reference_partner", "test_partner",
		"primary_domain", "country", "state_region", "city", "contact_name", "contact_email",
		"finance_contact_name", "finance_contact_email", "system_health", "platform_version",
		"created_at", "updated_at",
	})
	for rows.Next() {
		var id, displayName, legalName, categoryEN, categoryHU, lifecycle string
		var existing, reference, testPartner bool
		var primaryDomain, country, stateRegion, city, contactName, contactEmail string
		var financeName, financeEmail, health, platformVersion string
		var createdAt, updatedAt time.Time
		if rows.Scan(
			&id, &displayName, &legalName, &categoryEN, &categoryHU,
			&lifecycle, &existing, &reference, &testPartner,
			&primaryDomain, &country, &stateRegion, &city, &contactName, &contactEmail,
			&financeName, &financeEmail, &health, &platformVersion, &createdAt, &updatedAt,
		) != nil {
			continue
		}
		_ = writer.Write([]string{
			id, displayName, legalName, categoryEN, categoryHU, lifecycle,
			strconv.FormatBool(existing), strconv.FormatBool(reference), strconv.FormatBool(testPartner),
			primaryDomain, country, stateRegion, city, contactName, contactEmail,
			financeName, financeEmail, health, platformVersion,
			createdAt.UTC().Format(time.RFC3339), updatedAt.UTC().Format(time.RFC3339),
		})
	}
}
