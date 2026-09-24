#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

checks = {
    "services/cmd/tenantfinance/domain.go": [
        'sourceManual   = "MANUAL"',
        'sourceWorkflow = "WORKFLOW"',
        'sourceSchedule = "SCHEDULE"',
        'statusReadyForIssue = "READY_FOR_ISSUE"',
        'invoiceModuleKey = "invoice_documents"',
        "normalizeIssuer",
        "calculateItems",
    ],
    "services/cmd/tenantfinance/storage.go": [
        "tenant_finance.invoices",
        "tenant_finance.invoice_items",
        "tenant_finance.invoice_events",
        "tenant_finance_source_once_idx",
        "WHERE i.partner_id=$1 AND i.id=$2",
        "PARTNERS_SERVICE_CURRENT_SNAPSHOT",
        "automation.EnqueueTx",
        "tenant_invoice.ready_for_issue.v1",
    ],
    "services/cmd/tenantfinance/main.go": [
        "X-Himate-Partner-ID",
        "X-Himate-User-ID",
        "HIMATE_AUTOMATION_FINANCE_SECRET",
        "ensureAutomationSubscriptions",
        "runAutomationConsumer",
        "HIMATE_TENANT_FINANCE_DEFAULT_TERMS_DAYS",
        "HIMATE_TENANT_FINANCE_DEFAULT_ACCOUNTING_BASIS",
        "PARTNERS_HOSTPORT",
        "AUTOMATION_HOSTPORT",
    ],
    "services/cmd/tenantfinance/automation_consumer.go": [
        'workflowInvoiceReadyEvent = "workflow.billing_approved.v1"',
        'scheduleInvoiceReadyEvent = "scheduler.job_closed_invoice_ready.v1"',
        'event.ProducerService != "workshop"',
        'event.ProducerService != "scheduler"',
        'event.ModuleKey != "workshop_workflow"',
        'event.ModuleKey != "scheduler"',
        "PartnerID:        event.PartnerID",
        "SourceID:         event.SubjectID",
        "/internal/v1/automation/deliveries/claim",
        "createAutomatedReady",
        "ackAutomationDelivery",
        "failAutomationDelivery",
    ],
    "services/cmd/gateway/partner_user_modules.go": [
        'partnerInvoiceModuleKey     = "invoice_documents"',
        "a.requirePartnerModuleExecution",
        '"billing.write"',
        'a.hosts["tenant-finance"]',
        '"X-Himate-Partner-ID": u.PartnerID',
        '"X-Himate-User-ID": u.ID',
    ],
    "docker-compose.yml": [
        "tenantfinance:",
        "services/docker/tenantfinance.Dockerfile",
        "TENANT_FINANCE_HOSTPORT: tenantfinance:10000",
        "HIMATE_AUTOMATION_FINANCE_SECRET: finance-automation-secret-local-123456789",
    ],
    "render.yaml": [
        "name: himate-tenant-finance",
        "dockerfilePath: ./services/docker/tenantfinance.Dockerfile",
        "key: TENANT_FINANCE_HOSTPORT",
        "key: HIMATE_AUTOMATION_FINANCE_SECRET",
    ],
}

errors = []
for rel, needles in checks.items():
    path = ROOT / rel
    if not path.exists():
        errors.append(f"{rel}: missing")
        continue
    text = path.read_text()
    for needle in needles:
        if needle not in text:
            errors.append(f"{rel}: missing contract marker {needle!r}")

domain = (ROOT / "services/cmd/tenantfinance/domain.go").read_text()
main = (ROOT / "services/cmd/tenantfinance/main.go").read_text()
gateway = (ROOT / "services/cmd/gateway/partner_user_modules.go").read_text()
render = (ROOT / "render.yaml").read_text()

manual_struct = domain.split("type invoiceInput struct", 1)[1].split("}", 1)[0]
if "PartnerID string" in manual_struct:
    errors.append("manual invoice input must never accept partner_id from the client")
if "SourceType string" in manual_struct:
    errors.append("manual invoice input must never accept source_type from the client")

invoice_runtime = gateway.split("func (a *app) partnerInvoiceModuleRuntime", 1)[1]
invoice_runtime = invoice_runtime.split("func (a *app) applyPartnerUserModuleAccess", 1)[0]
if '"partner_id": u.PartnerID' in invoice_runtime:
    errors.append("gateway must not inject tenant identity into the invoice JSON body; use authoritative headers")

if "/internal/v1/tenant-finance/automation/invoice-intents" in main:
    errors.append("tenant finance must consume producer intents from the durable Automation bus, not a direct producer endpoint")
if "HIMATE_AUTOMATION_SERVICE_KEYS_JSON" in main:
    errors.append("tenant finance must not receive or parse the global automation verifier keyring")
if 'document_renderer": "DEFERRED"' not in main and 'document_renderer", "DEFERRED"' not in main:
    errors.append("tenant finance service must explicitly mark the legal PDF renderer as DEFERRED")

tf_render = render.split("name: himate-tenant-finance", 1)[1].split("\n  - type:", 1)[0]
if "HIMATE_AUTOMATION_SERVICE_KEYS_JSON" in tf_render:
    errors.append("tenant finance Render service must receive only its dedicated finance automation secret")
if "envVarKey: HIMATE_AUTOMATION_FINANCE_SECRET" not in tf_render:
    errors.append("tenant finance must bind the dedicated finance secret from the Automation service")

if errors:
    raise SystemExit("START-23.12 Phase 3B audit failed:\n- " + "\n- ".join(errors))

print("START-23.12 Phase 3B tenant invoicing architecture audit: PASS")
