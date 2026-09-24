package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"himate.local/services/internal/financepolicy"
)

const (
	sourceManual   = "MANUAL"
	sourceWorkflow = "WORKFLOW"
	sourceSchedule = "SCHEDULE"

	statusDraft         = "DRAFT"
	statusReadyForIssue = "READY_FOR_ISSUE"
	statusIssued        = "ISSUED"
	statusPaid          = "PAID"
	statusVoid          = "VOID"

	invoiceModuleKey = "invoice_documents"
	financeProducer  = "finance"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var invoicePrefixPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,11}$`)

type customerSnapshot struct {
	DisplayName  string `json:"display_name"`
	CompanyName  string `json:"company_name,omitempty"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	TaxID        string `json:"tax_id,omitempty"`
	Country      string `json:"country"`
	StateRegion  string `json:"state_region,omitempty"`
	City         string `json:"city"`
	PostalCode   string `json:"postal_code,omitempty"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2,omitempty"`
}

type issuerSnapshot struct {
	PartnerID          string `json:"partner_id"`
	DisplayName        string `json:"display_name"`
	LegalName          string `json:"legal_name"`
	BrandName          string `json:"brand_name,omitempty"`
	RegistrationNumber string `json:"registration_number,omitempty"`
	TaxID              string `json:"tax_id,omitempty"`
	Country            string `json:"country"`
	StateRegion        string `json:"state_region,omitempty"`
	City               string `json:"city"`
	PostalCode         string `json:"postal_code,omitempty"`
	AddressLine1       string `json:"address_line1"`
	AddressLine2       string `json:"address_line2,omitempty"`
	Website            string `json:"website,omitempty"`
	Phone              string `json:"phone,omitempty"`
	LogoURL            string `json:"logo_url"`
	FinanceEmail       string `json:"finance_contact_email,omitempty"`
}

type invoiceItemInput struct {
	Description    string `json:"description"`
	QuantityMilli  int64  `json:"quantity_milli"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	DiscountMinor  int64  `json:"discount_minor,omitempty"`
	TaxRateBPS     int    `json:"tax_rate_bps,omitempty"`
}

type calculatedItem struct {
	LineNo         int    `json:"line_no"`
	Description    string `json:"description"`
	QuantityMilli  int64  `json:"quantity_milli"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	DiscountMinor  int64  `json:"discount_minor"`
	TaxRateBPS     int    `json:"tax_rate_bps"`
	NetMinor       int64  `json:"net_minor"`
	TaxMinor       int64  `json:"tax_minor"`
	TotalMinor     int64  `json:"total_minor"`
}

type invoiceInput struct {
	RequestKey       string           `json:"request_key"`
	Currency         string           `json:"currency"`
	Customer         customerSnapshot `json:"customer"`
	Items            []invoiceItemInput `json:"items"`
	PaymentTermsDays *int             `json:"payment_terms_days,omitempty"`
	Notes            string           `json:"notes,omitempty"`
}

type automatedInvoiceIntent struct {
	PartnerID        string           `json:"partner_id"`
	SourceType       string           `json:"source_type"`
	SourceID         string           `json:"source_id"`
	Currency         string           `json:"currency"`
	Customer         customerSnapshot `json:"customer"`
	Items            []invoiceItemInput `json:"items"`
	PaymentTermsDays *int             `json:"payment_terms_days,omitempty"`
	Notes            string           `json:"notes,omitempty"`
	CorrelationID    string           `json:"correlation_id,omitempty"`
	CausationID      string           `json:"causation_id,omitempty"`
}

type financePolicyState struct {
	Policy        financepolicy.Policy `json:"policy"`
	DefaultCurrency string              `json:"default_currency"`
	InvoicePrefix   string              `json:"invoice_prefix"`
	Source           string              `json:"source"`
}

type invoiceRecord struct {
	ID                   string
	PartnerID            string
	SourceType           string
	SourceID             string
	RequestKey           string
	DraftHash            string
	Status               string
	InvoiceNumber        string
	InvoicePrefix        string
	Currency             string
	PaymentTermsOverride *int
	PaymentTermsDays     *int
	AccountingBasis      string
	PaymentMethods       []string
	Issuer               issuerSnapshot
	Customer             customerSnapshot
	SubtotalMinor        int64
	TaxMinor             int64
	TotalMinor           int64
	Notes                string
	CreatedBy            string
	FinalizedBy          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ReadyAt              *time.Time
	IssuedAt             *time.Time
	PaidAt               *time.Time
	Items                []calculatedItem
}

func newID(prefix string) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", prefix, time.Now().UTC().UnixNano())))
		buf = sum[:12]
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func normalizeCurrency(v, fallback string) (string, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		v = strings.ToUpper(strings.TrimSpace(fallback))
	}
	if !currencyPattern.MatchString(v) {
		return "", errors.New("currency must be a three-letter ISO-style code")
	}
	return v, nil
}

func normalizeCustomer(in customerSnapshot, final bool) (customerSnapshot, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.CompanyName = strings.TrimSpace(in.CompanyName)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	in.TaxID = strings.TrimSpace(in.TaxID)
	in.Country = strings.TrimSpace(in.Country)
	in.StateRegion = strings.TrimSpace(in.StateRegion)
	in.City = strings.TrimSpace(in.City)
	in.PostalCode = strings.TrimSpace(in.PostalCode)
	in.AddressLine1 = strings.TrimSpace(in.AddressLine1)
	in.AddressLine2 = strings.TrimSpace(in.AddressLine2)
	if in.DisplayName == "" && in.CompanyName == "" {
		return in, errors.New("customer display_name or company_name is required")
	}
	if in.DisplayName == "" {
		in.DisplayName = in.CompanyName
	}
	if final {
		if in.Country == "" || in.City == "" || in.AddressLine1 == "" {
			return in, errors.New("customer billing country, city and address_line1 are required before issue")
		}
		if strings.EqualFold(in.Country, "United States") || strings.EqualFold(in.Country, "US") || strings.EqualFold(in.Country, "USA") {
			if in.StateRegion == "" || in.PostalCode == "" {
				return in, errors.New("US customer billing state_region and postal_code are required before issue")
			}
		}
	}
	return in, nil
}

func normalizeIssuer(partnerID string, raw map[string]any) (issuerSnapshot, error) {
	value := func(key string) string {
		v, ok := raw[key]
		if !ok || v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(v))
	}
	out := issuerSnapshot{
		PartnerID:          strings.TrimSpace(partnerID),
		DisplayName:        value("display_name"),
		LegalName:          value("legal_name"),
		BrandName:          value("brand_name"),
		RegistrationNumber: value("registration_number"),
		TaxID:              value("tax_id"),
		Country:            value("country"),
		StateRegion:        value("state_region"),
		City:               value("city"),
		PostalCode:         value("postal_code"),
		AddressLine1:       value("address_line1"),
		AddressLine2:       value("address_line2"),
		Website:            value("website"),
		Phone:              value("phone"),
		LogoURL:            value("logo_url"),
		FinanceEmail:       strings.ToLower(value("finance_contact_email")),
	}
	if out.LegalName == "" {
		out.LegalName = out.DisplayName
	}
	if out.DisplayName == "" {
		out.DisplayName = out.LegalName
	}
	if out.PartnerID == "" || out.LegalName == "" || out.Country == "" || out.City == "" || out.AddressLine1 == "" {
		return out, errors.New("partner company profile is incomplete: legal name and billing address are required")
	}
	if strings.EqualFold(out.Country, "United States") || strings.EqualFold(out.Country, "US") || strings.EqualFold(out.Country, "USA") {
		if out.StateRegion == "" || out.PostalCode == "" {
			return out, errors.New("partner company profile is incomplete: US state_region and postal_code are required")
		}
	}
	if out.LogoURL == "" {
		return out, errors.New("partner company profile is incomplete: logo_url is required for invoice documents")
	}
	return out, nil
}

func safeRoundDiv(product, divisor int64) int64 {
	if product >= 0 {
		return (product + divisor/2) / divisor
	}
	return (product - divisor/2) / divisor
}

func calculateItems(inputs []invoiceItemInput) ([]calculatedItem, int64, int64, int64, error) {
	if len(inputs) == 0 {
		return nil, 0, 0, 0, errors.New("at least one invoice item is required")
	}
	if len(inputs) > 200 {
		return nil, 0, 0, 0, errors.New("an invoice may contain at most 200 items")
	}
	items := make([]calculatedItem, 0, len(inputs))
	var subtotal, taxTotal, total int64
	for i, in := range inputs {
		desc := strings.TrimSpace(in.Description)
		if desc == "" || len(desc) > 500 {
			return nil, 0, 0, 0, fmt.Errorf("item %d description is required and must not exceed 500 characters", i+1)
		}
		if in.QuantityMilli <= 0 || in.QuantityMilli > 100_000_000 {
			return nil, 0, 0, 0, fmt.Errorf("item %d quantity_milli is outside the supported range", i+1)
		}
		if in.UnitPriceMinor < 0 || in.UnitPriceMinor > 10_000_000_000 {
			return nil, 0, 0, 0, fmt.Errorf("item %d unit_price_minor is outside the supported range", i+1)
		}
		if in.TaxRateBPS < 0 || in.TaxRateBPS > 10_000 {
			return nil, 0, 0, 0, fmt.Errorf("item %d tax_rate_bps must be between 0 and 10000", i+1)
		}
		if in.QuantityMilli != 0 && in.UnitPriceMinor > math.MaxInt64/in.QuantityMilli {
			return nil, 0, 0, 0, fmt.Errorf("item %d amount is too large", i+1)
		}
		grossNet := safeRoundDiv(in.QuantityMilli*in.UnitPriceMinor, 1000)
		if in.DiscountMinor < 0 || in.DiscountMinor > grossNet {
			return nil, 0, 0, 0, fmt.Errorf("item %d discount exceeds the line amount", i+1)
		}
		net := grossNet - in.DiscountMinor
		if net != 0 && int64(in.TaxRateBPS) > math.MaxInt64/net {
			return nil, 0, 0, 0, fmt.Errorf("item %d tax amount is too large", i+1)
		}
		tax := safeRoundDiv(net*int64(in.TaxRateBPS), 10_000)
		lineTotal := net + tax
		if subtotal > math.MaxInt64-net || taxTotal > math.MaxInt64-tax || total > math.MaxInt64-lineTotal {
			return nil, 0, 0, 0, errors.New("invoice total is too large")
		}
		subtotal += net
		taxTotal += tax
		total += lineTotal
		items = append(items, calculatedItem{
			LineNo: i + 1, Description: desc, QuantityMilli: in.QuantityMilli, UnitPriceMinor: in.UnitPriceMinor,
			DiscountMinor: in.DiscountMinor, TaxRateBPS: in.TaxRateBPS, NetMinor: net, TaxMinor: tax, TotalMinor: lineTotal,
		})
	}
	return items, subtotal, taxTotal, total, nil
}

func draftEnvelopeHash(sourceType, sourceID, currency string, customer customerSnapshot, items []calculatedItem, terms *int, notes string) string {
	payload := map[string]any{
		"source_type": sourceType, "source_id": sourceID, "currency": currency, "customer": customer,
		"items": items, "payment_terms_days": terms, "notes": strings.TrimSpace(notes),
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func validateSourceForService(serviceID, sourceType string) error {
	serviceID = strings.TrimSpace(serviceID)
	sourceType = strings.ToUpper(strings.TrimSpace(sourceType))
	switch sourceType {
	case sourceWorkflow:
		if serviceID != "workshop" {
			return errors.New("WORKFLOW invoice intents may only be created by the workshop service identity")
		}
	case sourceSchedule:
		if serviceID != "scheduler" {
			return errors.New("SCHEDULE invoice intents may only be created by the scheduler service identity")
		}
	default:
		return errors.New("automated invoice source_type must be WORKFLOW or SCHEDULE")
	}
	return nil
}

func normalizeInvoicePrefix(v string) (string, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if !invoicePrefixPattern.MatchString(v) {
		return "", errors.New("invoice_prefix must contain 1-12 uppercase letters, digits, underscore or dash")
	}
	return v, nil
}

func invoicePayload(rec invoiceRecord) map[string]any {
	out := map[string]any{
		"id": rec.ID, "partner_id": rec.PartnerID, "source_type": rec.SourceType, "source_id": rec.SourceID,
		"request_key": rec.RequestKey, "status": rec.Status, "invoice_number": rec.InvoiceNumber, "invoice_prefix_snapshot": rec.InvoicePrefix, "currency": rec.Currency,
		"accounting_basis": rec.AccountingBasis, "payment_methods": rec.PaymentMethods,
		"issuer": rec.Issuer, "customer": rec.Customer,
		"subtotal_minor": rec.SubtotalMinor, "tax_minor": rec.TaxMinor, "total_minor": rec.TotalMinor,
		"notes": rec.Notes, "created_by": rec.CreatedBy, "finalized_by": rec.FinalizedBy,
		"created_at": rec.CreatedAt, "updated_at": rec.UpdatedAt, "items": rec.Items,
	}
	if rec.PaymentTermsOverride != nil {
		out["payment_terms_override_days"] = *rec.PaymentTermsOverride
	}
	if rec.PaymentTermsDays != nil {
		out["payment_terms_days"] = *rec.PaymentTermsDays
	}
	if rec.ReadyAt != nil {
		out["ready_at"] = *rec.ReadyAt
	}
	if rec.IssuedAt != nil {
		out["issued_at"] = *rec.IssuedAt
	}
	if rec.PaidAt != nil {
		out["paid_at"] = *rec.PaidAt
	}
	out["document_state"] = map[string]any{
		"renderer": "DEFERRED", "legal_document_generated": rec.Status == statusIssued || rec.Status == statusPaid,
		"rule": "READY_FOR_ISSUE is a frozen finance snapshot; ISSUED is reserved for the future compliant document renderer",
	}
	return out
}
