#!/usr/bin/env sh
# Shared real file-backed Evidence fixture helpers for historical and current Compose acceptance.
# Requires BASE_URL to be set by the caller.

create_pdf_evidence() {
  cookie="$1"
  partner_id="$2"
  evidence_type="$3"
  title="$4"
  tag="$5"
  metric_key="${6:-}"

  tmp_root="${TMPDIR:-/tmp}"
  pdf="$tmp_root/himate-evidence-${tag}-$$.pdf"
  # http.DetectContentType identifies this as application/pdf; the payload is deterministic
  # enough for acceptance while still being a real binary object persisted through Storage.
  cat >"$pdf" <<'PDF'
%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 37 >>
stream
BT /F1 12 Tf 20 100 Td (HIMATE) Tj ET
endstream
endobj
xref
0 5
0000000000 65535 f
trailer
<< /Root 1 0 R /Size 5 >>
startxref
0
%%EOF
PDF

  response="$(curl -fsS -b "$cookie"     -F "partner_id=$partner_id"     -F "metric_key="     -F "evidence_type=$evidence_type"     -F "title=$title"     -F "description=Acceptance file-backed Evidence fixture"     -F "period_start="     -F "period_end="     -F "file=@$pdf;type=application/pdf"     "$BASE_URL/api/v1/evidence")"
  rm -f "$pdf"

  evidence_id="$(printf '%s' "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["has_file"] is True; assert d["sha256"]; assert int(d["size_bytes"])>0; print(d["id"])')"
  test -n "$evidence_id"

  integrity="$(curl -fsS -b "$cookie" "$BASE_URL/api/v1/evidence/$evidence_id/integrity")"
  printf '%s' "$integrity" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["valid"] is True; assert d["status"]=="VALID"; assert d["sha256"]; assert int(d["size_bytes"])>0'
  printf '%s' "$evidence_id"
}

register_billing_evidence_document() {
  cookie="$1"
  partner_id="$2"
  billing_kind="$3"
  evidence_type="$4"
  title="$5"
  tag="$6"

  evidence_id="$(create_pdf_evidence "$cookie" "$partner_id" "$evidence_type" "$title" "$tag")"
  payload="$(python3 - "$billing_kind" "$title" "$evidence_id" <<'PY'
import json,sys
kind,title,evidence_id=sys.argv[1:]
print(json.dumps({
  "kind":kind,
  "name":title,
  "storage_url":"evidence://"+evidence_id,
  "note":"File-backed Evidence acceptance fixture"
},separators=(",",":")))
PY
)"
  response="$(curl -fsS -b "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/billing/partners/$partner_id/documents")"
  printf '%s' "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["storage_url"].startswith("evidence://"); print(d["id"])'
}
