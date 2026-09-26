package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func (a *app) exportImpactPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	metricKey := strings.TrimSpace(r.URL.Query().Get("metric_key"))
	query := `SELECT
		v.partner_id,v.metric_key,d.label_en,d.unit,d.aggregation,
		v.period_start,v.period_end,v.numeric_value,v.text_value,v.provenance,v.recorded_at
		FROM impact.metric_values v
		JOIN impact.metric_definitions d ON d.metric_key=v.metric_key
		WHERE 1=1`
	args := []any{}
	if partnerID != "" { args=append(args,partnerID); query += fmt.Sprintf(" AND v.partner_id=$%d",len(args)) }
	if metricKey != "" { args=append(args,metricKey); query += fmt.Sprintf(" AND v.metric_key=$%d",len(args)) }
	query += " ORDER BY v.period_end DESC,v.partner_id,v.metric_key,v.id DESC"
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil { common.APIError(w,500,"DB","Could not export impact data"); return }
	defer rows.Close()
	tableRows := make([][]string,0,64)
	for rows.Next() {
		var partner,key,label,unit,aggregation,textValue,provenance string
		var start,end,recordedAt time.Time
		var numeric sql.NullFloat64
		if rows.Scan(&partner,&key,&label,&unit,&aggregation,&start,&end,&numeric,&textValue,&provenance,&recordedAt)!=nil { continue }
		value:=textValue
		if numeric.Valid { value=fmt.Sprintf("%.2f",numeric.Float64) }
		tableRows=append(tableRows,[]string{partner,label,key,value,unit,aggregation,start.Format("2006-01-02"),end.Format("2006-01-02"),provenance})
	}
	common.WriteBrandedTablePDF(w,"himate-impact.pdf","HiMate Central - Impact","Auditable metric value export",
		[]string{"Partner","Metric","Key","Value","Unit","Aggregation","From","To","Provenance"},tableRows)
}
