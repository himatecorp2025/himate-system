package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func (a *app) exportImpactCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	metricKey := strings.TrimSpace(r.URL.Query().Get("metric_key"))
	query := `SELECT
		v.partner_id,v.metric_key,d.label_en,d.label_hu,d.unit,d.aggregation,
		v.period_start,v.period_end,v.numeric_value,v.text_value,v.provenance,
		v.source_ref,v.evidence_id,v.recorded_by,v.recorded_at
		FROM impact.metric_values v
		JOIN impact.metric_definitions d ON d.metric_key=v.metric_key
		WHERE 1=1`
	args := []any{}
	if partnerID != "" {
		args = append(args, partnerID)
		query += fmt.Sprintf(" AND v.partner_id=$%d", len(args))
	}
	if metricKey != "" {
		args = append(args, metricKey)
		query += fmt.Sprintf(" AND v.metric_key=$%d", len(args))
	}
	query += " ORDER BY v.period_end DESC,v.partner_id,v.metric_key,v.id DESC"
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not export impact data")
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="himate-impact.csv"`)
	w.Header().Set("Cache-Control", "private, no-store")
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{
		"partner_id", "metric_key", "label_en", "label_hu", "unit", "aggregation",
		"period_start", "period_end", "numeric_value", "text_value", "provenance",
		"source_ref", "evidence_id", "recorded_by", "recorded_at",
	})
	for rows.Next() {
		var partner, key, labelEN, labelHU, unit, aggregation string
		var start, end, recordedAt time.Time
		var numeric sql.NullFloat64
		var textValue, provenance, sourceRef, evidenceID, recordedBy string
		if rows.Scan(
			&partner, &key, &labelEN, &labelHU, &unit, &aggregation,
			&start, &end, &numeric, &textValue, &provenance,
			&sourceRef, &evidenceID, &recordedBy, &recordedAt,
		) != nil {
			continue
		}
		numericValue := ""
		if numeric.Valid {
			numericValue = fmt.Sprintf("%.4f", numeric.Float64)
		}
		_ = writer.Write([]string{
			partner, key, labelEN, labelHU, unit, aggregation,
			start.Format("2006-01-02"), end.Format("2006-01-02"), numericValue,
			textValue, provenance, sourceRef, evidenceID, recordedBy,
			recordedAt.UTC().Format(time.RFC3339),
		})
	}
}
