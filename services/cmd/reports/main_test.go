package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRenderPDFProducesSelfContainedPDF(t *testing.T) {
	snapshot := map[string]any{
		"report_id": "rpt_test",
		"title": "Partner Impact Report",
		"period_start": "2026-01-01",
		"period_end": "2026-01-31",
		"disclaimer": "Not a legal determination.",
		"metric_sections": []any{
			map[string]any{
				"partner_id": "ptr_test",
				"metrics": []any{
					map[string]any{"label": "Events", "unit": "count", "numeric_value": float64(12)},
				},
			},
		},
		"evidence": []any{map[string]any{"id":"evd_1","evidence_type":"PDF","verification_status":"VERIFIED","title":"Receipt"}},
		"data_sources": []any{map[string]any{"metric_key":"culture.events","provenance":"VERIFIED_DOCUMENT","source_ref":"evd_1"}},
	}
	raw := renderPDF(snapshot)
	if !bytes.HasPrefix(raw, []byte("%PDF-1.4")) {
		t.Fatal("generated report is not a PDF")
	}
	if !bytes.Contains(raw, []byte("rpt_test")) || !bytes.Contains(raw, []byte("Evidence")) {
		t.Fatal("generated PDF is missing report content")
	}
	if !bytes.HasSuffix(raw, []byte("%%EOF\n")) {
		t.Fatal("generated PDF is missing EOF marker")
	}
}

func TestSnapshotSectionsReadsMetrics(t *testing.T) {
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(`{"metric_sections":[{"partner_id":"ptr_1","metrics":[{"label":"Events","unit":"count","numeric_value":7}]}]}`), &snapshot); err != nil {
		t.Fatal(err)
	}
	sections := snapshotSections(snapshot)
	if len(sections) != 1 || len(sections[0].Metrics) != 1 || !sections[0].Metrics[0].HasValue {
		t.Fatalf("unexpected parsed sections: %#v", sections)
	}
}
