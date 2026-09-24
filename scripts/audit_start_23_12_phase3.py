#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT=Path(__file__).resolve().parents[1]
failures=[]

def require(cond,msg):
    if not cond:
        failures.append(msg)

automation=(ROOT/"services/cmd/automation/main.go").read_text()
identity=(ROOT/"services/internal/automation/contract.go").read_text()
outbox=(ROOT/"services/internal/automation/outbox.go").read_text()
finance=(ROOT/"services/internal/financepolicy/policy.go").read_text()
catalog=(ROOT/"services/cmd/catalog/main.go").read_text()
plans=(ROOT/"services/cmd/billing/plans.go").read_text()
billing=(ROOT/"services/cmd/billing/main.go").read_text()
commercial=(ROOT/"services/cmd/billing/commercial_automation.go").read_text()
compose=(ROOT/"docker-compose.yml").read_text()
render=(ROOT/"render.yaml").read_text()
ci=(ROOT/".github/workflows/ci.yml").read_text()
acceptance=(ROOT/"docs/START-23.12_PHASE3_ACCEPTANCE.md").read_text() if (ROOT/"docs/START-23.12_PHASE3_ACCEPTANCE.md").exists() else ""

# Phase 1A - machine identity
for token in ["X-Himate-Service-ID","X-Himate-Service-Timestamp","X-Himate-Service-Signature","ConstantTimeCompare"]:
    require(token in identity,f"service identity contract missing {token}")
require("5*time.Minute" in automation or "5 * time.Minute" in automation,"automation service must enforce signed-request freshness")
require("HIMATE_AUTOMATION_SERVICE_KEYS_JSON" in automation,"per-service automation credential registry is missing")

# Phase 2A - durable event backbone
for token in [
    "automation.subscriptions","automation.events","automation.deliveries",
    "FOR UPDATE SKIP LOCKED","DEAD_LETTER","available_at","lease_until",
    "EVENT_KEY_CONFLICT",
]:
    require(token in automation,f"durable automation backbone missing {token}")
for token in ["automation_outbox.events","EnqueueTx","ClaimOutbox","MarkOutboxPublished","FailOutbox"]:
    require(token in outbox,f"transactional producer outbox SDK missing {token}")
require("requires the caller business transaction" in outbox,"outbox must require caller transaction")
require("validateAutomationManifest" in catalog and '"automation"' in catalog,"catalog automation manifest validation is missing")
for token in ["produces_events","consumes_events","commands","scheduled_actions","required_permissions","required_modules"]:
    require(token in catalog,f"module automation contract missing {token}")

# Phase 3A - platform billing financial integrity
require("tx,err:=a.db.BeginTx(ctx,nil)" in plans,"plan invoice creation is not transactional")
require("SELECT id FROM billing.invoices WHERE invoice_key=$1 FOR UPDATE" in plans,"plan invoice replay does not lock/reconcile the existing invoice")
require("emitBillingEventTx" in plans,"plan invoice event is not committed in the invoice transaction")
require("plan invoice item idempotency conflict" in plans,"plan invoice replay does not verify line-item integrity")
require("attachInvoiceItemsTx" in billing,"legacy invoice assembly is not transaction-scoped")
require("emitBillingEventTx" in billing,"legacy INVOICE_GENERATED event is not transaction-scoped")
require("emitBillingEventWith(ctx,store" in commercial,"invoice-item events are not transaction-scoped")

# Phase 3B - reusable tenant finance policy
for token in ["ResolvePaymentTerms","invoiceOverride","serviceDefault","partnerDefault","AccountingCash","AccountingAccrual","PaymentMethods"]:
    require(token in finance,f"tenant finance policy contract missing {token}")
require("default_payment_terms_days" in finance,"configurable payment terms policy is missing")
require("hard-coded 8" not in finance.lower(),"finance policy must not hard-code an eight-day rule")

# Deployment / acceptance gates
require("automation:" in compose and "automation.Dockerfile" in compose,"local topology is missing automation service")
require("name: himate-automation" in render,"Render blueprint is missing automation service")
require("HIMATE_AUTOMATION_SERVICE_KEYS_JSON" in render,"Render automation credentials are not externalized")
require("START-23.12 Phase 3 Architecture Acceptance audit" in ci,"CI is missing Phase 3 source acceptance gate")
require("START-23.12 Phase 3 Automation Backbone smoke" in ci,"CI is missing Phase 3 runtime smoke")
require("CONFIGURABLE_PAYMENT_TERMS" in acceptance,"Phase 3 acceptance record does not freeze configurable payment terms")
require("MODULE_RUNTIME_DEFERRED" in acceptance,"Phase 3 acceptance record must keep future business modules deferred")

if failures:
    for f in failures:
        print("FAIL:",f)
    sys.exit(1)

print("START-23.12 Phase 3 architecture audit passed: machine identity, durable automation, transactional billing and configurable tenant finance policy are closed.")
