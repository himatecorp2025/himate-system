#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

central6 = read("services/cmd/billing/central6.go")
billing = read("services/cmd/billing/main.go")
plans = read("services/cmd/billing/plans.go")
payments = read("services/cmd/billing/payment_provider.go")
dunning = read("services/cmd/billing/dunning.go")
gateway = read("services/cmd/gateway/main.go")
partner_gateway = read("services/cmd/gateway/partner_portal.go")
ui = read("frontend/lib/main.dart")
partner_ui = read("frontend/lib/partner_portal.dart")
localization = read("frontend/lib/localization.dart")
openapi = read("docs/openapi.yaml")
ci = read(".github/workflows/ci.yml")

require(re.search(r"Version:\s+18\b", central6) is not None, "Central-6 billing migration version 18 missing")

for token in [
    "central-6-finance-onboarding-invoice-approval",
    "billing.finance_transactions",
    "billing.invoice_delivery_outbox",
    "billing.partner_onboarding",
    "billing.partner_onboarding_history",
    "billing.partner_support_waivers",
    "billing_finance_transactions_invoice_state_unique",
]:
    require(token in central6, f"Central-6 schema contract missing: {token}")

for token in [
    'onboardingRegistered     = "REGISTERED"',
    'onboardingPendingReview  = "PENDING_REVIEW"',
    'onboardingClassified     = "CLASSIFIED"',
    'onboardingInvoicePending = "INVOICE_PENDING"',
    'onboardingPaymentPending = "PAYMENT_PENDING"',
    'onboardingAdminApproval  = "ADMIN_APPROVAL"',
    'onboardingActive         = "ACTIVE"',
    "central6OnboardingTransitionAllowed",
    "onboardingPrerequisite",
    "portalEnabled := nextState == onboardingActive",
]:
    require(token in central6, f"Central-6 onboarding state machine missing: {token}")

for token in [
    'classificationPaid         = "PAID"',
    'classificationCharity      = "CHARITY"',
    'classificationSponsored    = "SPONSORED"',
    'classificationComplimentary = "COMPLIMENTARY"',
    "zeroDollarClassification",
    "documented zero-dollar waiver/support approval is required",
    "ZERO_DOLLAR_NO_INVOICE",
]:
    require(token in central6, f"Central-6 classification/zero-dollar rule missing: {token}")

for token in [
    'invoiceDraft     = "DRAFT"',
    'invoiceApproved  = "APPROVED"',
    'invoiceSent      = "SENT"',
    'invoicePaid      = "PAID"',
    'invoiceCancelled = "CANCELLED"',
    'case "approve":',
    'case "send":',
    'case "mark-paid":',
    'case "cancel":',
    "invoicePDF",
    "basicPDF",
]:
    require(token in central6, f"Central-6 invoice workflow missing: {token}")

for token in [
    'mux.HandleFunc("/api/v1/billing/finance/overview", a.financeOverview)',
    'mux.HandleFunc("/api/v1/billing/invoices", a.invoiceCollection)',
    'mux.HandleFunc("/api/v1/billing/invoices/", a.invoiceByID)',
    "central6BillingMigration()",
    'parts[1] == "portal-gate"',
    'section == "onboarding"',
    'partner_visible',
]:
    require(token in billing, f"Central-6 billing route/read-model integration missing: {token}")

for token in [
    "workflow_status,source",
    "'DRAFT','AUTOMATED'",
    '"INVOICE","DRAFT"',
]:
    require(token in plans, f"recurring plan invoices must start as approval drafts: {token}")

require("workflow_status='PAID'" in payments, "provider settlement must synchronize invoice workflow to PAID")
require("workflow_status IN ('SENT','PAID')" in dunning, "dunning must ignore DRAFT/APPROVED invoices")

for token in [
    'strings.Contains(path,"/onboarding")',
    'strings.HasPrefix(path,"/api/v1/billing/invoices/")',
]:
    require(token in gateway, f"Central-6 approval permission gate missing: {token}")

for token in [
    '"/internal/v1/partners/"+url.PathEscape(partnerID)+"/portal-gate"',
    "a.internalToken",
    "/invoices?partner_visible=true",
    "partnerInvoicePDF",
]:
    require(token in partner_gateway, f"Partner Portal Central-6 gate/PDF integration missing: {token}")

for token in [
    "class FinancePage",
    "Pending onboarding",
    "Approve invoice",
    "Send invoice",
    "Mark paid",
    "Cancel invoice",
    "Partner onboarding",
    "workflow_status",
    "/api/v1/billing/finance/overview",
]:
    require(token in ui, f"Central-6 Finance UI missing: {token}")

for token in [
    "Only approved and distributed invoices appear here",
    "/partner/api/v1/billing/invoices/$id/pdf",
    "workflow_status",
]:
    require(token in partner_ui, f"Partner Portal invoice distribution UI missing: {token}")

for token in [
    "'Pending onboarding':",
    "'Approve invoice':",
    "'Send invoice':",
    "'Mark paid':",
    "'Cancel invoice':",
]:
    require(token in localization, f"Central-6 Hungarian localization missing: {token}")

for path in [
    "/api/v1/billing/finance/overview:",
    "/api/v1/billing/invoices:",
    "/api/v1/billing/invoices/{invoiceId}:",
    "/api/v1/billing/invoices/{invoiceId}/approve:",
    "/api/v1/billing/invoices/{invoiceId}/send:",
    "/api/v1/billing/invoices/{invoiceId}/mark-paid:",
    "/api/v1/billing/invoices/{invoiceId}/cancel:",
    "/api/v1/billing/invoices/{invoiceId}/pdf:",
    "/api/v1/billing/partners/{partnerId}/onboarding:",
    "/partner/api/v1/billing/invoices/{invoiceId}/pdf:",
]:
    require(path in openapi, f"OpenAPI Central-6 contract missing: {path}")

for forbidden in [
    "Partner Portal access is available immediately after registration",
    "DRAFT invoices are visible in Partner Portal",
    "zero-dollar invoice",
]:
    require(forbidden not in openapi + "\n" + ui + "\n" + partner_ui, f"retired Central-6 behavior survived: {forbidden}")

require("audit_central_6.py" in ci, "Central-6 static audit is not wired into CI")
require("smoke_central_6.sh" in ci, "Central-6 runtime smoke is not wired into CI")

print("Central-6 Licensing/Finance/Onboarding acceptance: PASS")
