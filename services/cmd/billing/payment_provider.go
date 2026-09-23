package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"math"
	"net/http"
	"strings"
	"time"
)

func start234BillingPaymentMigration() common.Migration {
	return common.Migration{
		Version: 8,
		Name: "start-23-4-provider-backed-payments",
		Statements: []string{
			`ALTER TABLE billing.initial_licenses ADD COLUMN IF NOT EXISTS payment_attempt_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.initial_licenses ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.initial_licenses ADD COLUMN IF NOT EXISTS provider_payment_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS payment_attempt_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS provider_payment_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS payment_failure_code TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS payment_failure_message TEXT NOT NULL DEFAULT ''`,
			`CREATE TABLE IF NOT EXISTS billing.payment_settlements(
				provider_event_id TEXT PRIMARY KEY,
				attempt_id TEXT NOT NULL,
				partner_id TEXT NOT NULL,
				invoice_id TEXT NOT NULL DEFAULT '',
				purpose TEXT NOT NULL,
				amount NUMERIC(12,2) NOT NULL,
				currency TEXT NOT NULL,
				status TEXT NOT NULL,
				provider TEXT NOT NULL,
				provider_payment_id TEXT NOT NULL,
				failure_code TEXT NOT NULL DEFAULT '',
				failure_message TEXT NOT NULL DEFAULT '',
				settled_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS billing_payment_settlement_attempt_success
				ON billing.payment_settlements(attempt_id) WHERE status='SUCCEEDED'`,
		},
	}
}

type paymentSettlementInput struct {
	AttemptID         string  `json:"attempt_id"`
	PartnerID         string  `json:"partner_id"`
	InvoiceID         string  `json:"invoice_id"`
	Purpose           string  `json:"purpose"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	Status            string  `json:"status"`
	Provider          string  `json:"provider"`
	ProviderPaymentID string  `json:"provider_payment_id"`
	ProviderEventID   string  `json:"provider_event_id"`
	FailureCode       string  `json:"failure_code"`
	FailureMessage    string  `json:"failure_message"`
}

func (a *app) paymentSettlement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, 405, "METHOD", "Use POST")
		return
	}
	var in paymentSettlementInput
	if common.Decode(r, &in) != nil {
		common.APIError(w, 400, "JSON", "Invalid settlement")
		return
	}
	in.AttemptID = strings.TrimSpace(in.AttemptID)
	in.PartnerID = strings.TrimSpace(in.PartnerID)
	in.InvoiceID = strings.TrimSpace(in.InvoiceID)
	in.Purpose = strings.ToUpper(strings.TrimSpace(in.Purpose))
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.Status = strings.ToUpper(strings.TrimSpace(in.Status))
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.ProviderPaymentID = strings.TrimSpace(in.ProviderPaymentID)
	in.ProviderEventID = strings.TrimSpace(in.ProviderEventID)
	if in.AttemptID == "" || in.PartnerID == "" || in.ProviderEventID == "" || in.ProviderPaymentID == "" ||
		in.Amount <= 0 || len(in.Currency) != 3 || (in.Status != "SUCCEEDED" && in.Status != "FAILED") {
		common.APIError(w, 400, "VALIDATION", "Settlement identity, amount, currency and provider evidence are required")
		return
	}
	if in.Purpose != "ACTIVATION_LICENSE" && in.Purpose != "INVOICE" {
		common.APIError(w, 400, "VALIDATION", "Unknown settlement purpose")
		return
	}

	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
	var dunningEligibleInvoice bool
	var dunningAttempts int
	var dunningInvoiceDate time.Time
	var dunningPreState string
	if err != nil {
		common.APIError(w, 500, "DB", "Could not start payment settlement")
		return
	}
	defer tx.Rollback()

	var exists bool
	if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM billing.payment_settlements WHERE provider_event_id=$1)`, in.ProviderEventID).Scan(&exists); err != nil {
		common.APIError(w, 500, "DB", "Could not verify payment settlement")
		return
	}
	if exists {
		common.JSON(w, 200, map[string]any{"status":"duplicate","provider_event_id":in.ProviderEventID})
		return
	}

	if in.Purpose == "ACTIVATION_LICENSE" {
		var required float64
		var currency, status string
		var waived bool
		if err = tx.QueryRow(`SELECT required_amount,currency,status,waived FROM billing.initial_licenses WHERE partner_id=$1 FOR UPDATE`, in.PartnerID).
			Scan(&required, &currency, &status, &waived); err != nil {
			common.APIError(w, 404, "LICENSE_NOT_FOUND", "Activation license not found")
			return
		}
		if waived {
			common.APIError(w, 409, "LICENSE_WAIVED", "Waived activation license must not receive a provider settlement")
			return
		}
		if !strings.EqualFold(currency, in.Currency) || math.Abs(required-in.Amount) > 0.005 {
			common.APIError(w, 409, "SETTLEMENT_MISMATCH", "Provider settlement does not match the activation license amount and currency")
			return
		}
		if in.Status == "SUCCEEDED" {
			_, err = tx.Exec(`UPDATE billing.initial_licenses SET paid_amount=required_amount,status='PAID',payment_date=CURRENT_DATE,
				payment_reference=$2,verified_by='payments-service',payment_attempt_id=$3,provider=$4,provider_payment_id=$5,updated_at=NOW()
				WHERE partner_id=$1`, in.PartnerID, in.ProviderPaymentID, in.AttemptID, in.Provider, in.ProviderPaymentID)
			if err == nil && status != "PAID" {
				err = emitBillingEventTx(r.Context(), tx, "LICENSE_PAID:"+in.ProviderEventID, in.PartnerID, "", "LICENSE_PAID", time.Now().UTC(), map[string]any{
					"amount":in.Amount,"currency":in.Currency,"provider":in.Provider,"provider_payment_id":in.ProviderPaymentID,
					"attempt_id":in.AttemptID,"provider_event_id":in.ProviderEventID,
				})
			}
		} else {
			err = emitBillingEventTx(r.Context(), tx, "LICENSE_PAYMENT_FAILED:"+in.ProviderEventID, in.PartnerID, "", "LICENSE_PAYMENT_FAILED", time.Now().UTC(), map[string]any{
				"amount":in.Amount,"currency":in.Currency,"provider":in.Provider,"attempt_id":in.AttemptID,
				"failure_code":in.FailureCode,"failure_message":in.FailureMessage,
			})
		}
		if err != nil { common.APIError(w, 500, "DB", "Could not settle activation license"); return }
	} else {
		var amount float64
		var currency, oldStatus, billingModel, chargeType string
		if err = tx.QueryRow(`SELECT total,currency,status,billing_model,charge_type,collection_attempts,invoice_date,dunning_state
			FROM billing.invoices WHERE id=$1 AND partner_id=$2 FOR UPDATE`, in.InvoiceID, in.PartnerID).
			Scan(&amount, &currency, &oldStatus, &billingModel, &chargeType, &dunningAttempts, &dunningInvoiceDate, &dunningPreState); err != nil {
			common.APIError(w, 404, "INVOICE_NOT_FOUND", "Invoice not found")
			return
		}
		dunningEligibleInvoice = dunningEligible(billingModel, chargeType)
		if !strings.EqualFold(currency, in.Currency) || math.Abs(amount-in.Amount) > 0.005 {
			common.APIError(w, 409, "SETTLEMENT_MISMATCH", "Provider settlement does not match the invoice total and currency")
			return
		}
		if in.Status == "SUCCEEDED" {
			_, err = tx.Exec(`UPDATE billing.invoices SET status='PAID',provider_status='SUCCEEDED',payment_attempt_id=$2,provider=$3,
				provider_payment_id=$4,paid_at=COALESCE(paid_at,NOW()),payment_failure_code='',payment_failure_message='' WHERE id=$1`,
				in.InvoiceID, in.AttemptID, in.Provider, in.ProviderPaymentID)
			if err == nil && oldStatus != "PAID" {
				err = emitBillingEventTx(r.Context(), tx, "INVOICE_PAID:"+in.ProviderEventID, in.PartnerID, "", "INVOICE_PAID", time.Now().UTC(), map[string]any{
					"invoice_id":in.InvoiceID,"amount":in.Amount,"currency":in.Currency,"provider":in.Provider,
					"provider_payment_id":in.ProviderPaymentID,"attempt_id":in.AttemptID,"provider_event_id":in.ProviderEventID,
				})
			}
		} else {
			_, err = tx.Exec(`UPDATE billing.invoices SET provider_status='FAILED',payment_attempt_id=$2,provider=$3,provider_payment_id=$4,
				payment_failure_code=$5,payment_failure_message=$6 WHERE id=$1`,
				in.InvoiceID, in.AttemptID, in.Provider, in.ProviderPaymentID, in.FailureCode, in.FailureMessage)
			if err == nil {
				err = emitBillingEventTx(r.Context(), tx, "INVOICE_PAYMENT_FAILED:"+in.ProviderEventID, in.PartnerID, "", "INVOICE_PAYMENT_FAILED", time.Now().UTC(), map[string]any{
					"invoice_id":in.InvoiceID,"amount":in.Amount,"currency":in.Currency,"attempt_id":in.AttemptID,
					"failure_code":in.FailureCode,"failure_message":in.FailureMessage,
				})
			}
		}
		if err != nil { common.APIError(w, 500, "DB", "Could not settle invoice payment"); return }
	}

	_, err = tx.Exec(`INSERT INTO billing.payment_settlements(provider_event_id,attempt_id,partner_id,invoice_id,purpose,amount,currency,status,provider,provider_payment_id,failure_code,failure_message)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		in.ProviderEventID,in.AttemptID,in.PartnerID,in.InvoiceID,in.Purpose,in.Amount,in.Currency,in.Status,in.Provider,in.ProviderPaymentID,in.FailureCode,in.FailureMessage)
	if err != nil { common.APIError(w, 500, "DB", "Could not persist payment settlement"); return }
	if err = tx.Commit(); err != nil { common.APIError(w, 500, "DB", "Could not commit payment settlement"); return }

	if in.Purpose == "INVOICE" {
		if dunningEligibleInvoice {
			if in.Status == "SUCCEEDED" {
				if dunningPreState == "PAST_DUE" || dunningPreState == "SUSPENDED" {
					_ = a.recoverDunningPayment(r.Context(), in.InvoiceID, in.PartnerID, time.Now().UTC())
				} else {
					_, _ = a.db.ExecContext(r.Context(), `UPDATE billing.partner_plan_subscriptions SET status='ACTIVE',updated_at=NOW()
						WHERE partner_id=$1 AND status='PAST_DUE'`, in.PartnerID)
				}
			} else if dunningAttempts >= dunningMaxAttempts {
				_ = a.suspendForNonPayment(r.Context(), in.InvoiceID, in.PartnerID, time.Now().UTC())
			} else {
				_ = a.markPastDue(r.Context(), in.InvoiceID, in.PartnerID, dunningAttempts, dunningInvoiceDate)
			}
		}
	}
	common.JSON(w, 200, map[string]any{"status":"settled","purpose":in.Purpose,"provider_event_id":in.ProviderEventID})
}

func (a *app) requestPaymentCharge(ctx context.Context, partnerID, invoiceID, purpose string, amount float64, currency, idempotency string) (map[string]any, error) {
	if strings.TrimSpace(a.paymentsHost) == "" { return nil, fmt.Errorf("PAYMENTS_HOSTPORT is required") }
	payload := map[string]any{
		"partner_id":partnerID,"invoice_id":invoiceID,"purpose":purpose,
		"amount":math.Round(amount*100)/100,"currency":currency,"idempotency_key":idempotency,
	}
	raw,_:=json.Marshal(payload)
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.paymentsHost+"/internal/v1/charges",bytes.NewReader(raw))
	if err!=nil{return nil,err}
	req.Header.Set("Content-Type","application/json")
	req.Header.Set("X-Himate-Internal-Token",a.token)
	resp,err:=a.client.Do(req)
	if err!=nil{return nil,err}
	defer resp.Body.Close()
	var out map[string]any
	if resp.StatusCode<200||resp.StatusCode>=300{
		return nil,fmt.Errorf("payments service status %d",resp.StatusCode)
	}
	if err:=json.NewDecoder(resp.Body).Decode(&out);err!=nil{return nil,err}
	return out,nil
}

func (a *app) collectActivationLicense(w http.ResponseWriter, r *http.Request, partnerID string) {
	if r.Method != http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	x,err:=a.ensureLicense(partnerID)
	if err!=nil{common.APIError(w,500,"DB","Could not load activation license");return}
	if x.Waived{common.APIError(w,409,"LICENSE_WAIVED","Waived license does not require payment");return}
	if x.Status=="PAID"{common.APIError(w,409,"LICENSE_ALREADY_PAID","Activation license is already paid");return}
	if x.Required<=0{common.APIError(w,409,"LICENSE_AMOUNT_REQUIRED","Activation license amount must be configured first");return}
	key:=fmt.Sprintf("activation-license:%s:%.2f:%s",partnerID,x.Required,x.Currency)
	out,err:=a.requestPaymentCharge(r.Context(),partnerID,"","ACTIVATION_LICENSE",x.Required,x.Currency,key)
	if err!=nil{common.APIError(w,502,"PAYMENT_PROVIDER","Could not initiate activation license payment");return}
	common.JSON(w,201,out)
}

func (a *app) retryPendingInvoiceCollections(ctx context.Context) error {
	rows,err:=a.db.QueryContext(ctx,`SELECT id,partner_id,currency,total FROM billing.invoices
		WHERE status<>'PAID' AND provider_status='COLLECTION_PENDING'
		ORDER BY invoice_date,id LIMIT 500`)
	if err!=nil{return err}
	defer rows.Close()
	type pending struct{ id,partnerID,currency string; total float64 }
	items:=[]pending{}
	for rows.Next(){
		var x pending
		if err:=rows.Scan(&x.id,&x.partnerID,&x.currency,&x.total);err!=nil{return err}
		items=append(items,x)
	}
	if err:=rows.Err();err!=nil{return err}
	for _,x:=range items{
		a.queueInvoiceCollection(ctx,x.id,x.partnerID,x.currency,x.total)
	}
	return nil
}

func (a *app) queueInvoiceCollection(ctx context.Context, invoiceID, partnerID, currency string, total float64) {
	out,err:=a.requestPaymentCharge(ctx,partnerID,invoiceID,"INVOICE",total,currency,"invoice:"+invoiceID)
	if err!=nil{
		_,_=a.db.ExecContext(ctx,`UPDATE billing.invoices SET provider_status='COLLECTION_PENDING' WHERE id=$1 AND status<>'PAID'`,invoiceID)
		return
	}
	attemptID:=strings.TrimSpace(fmt.Sprint(out["id"]))
	providerID:=strings.TrimSpace(fmt.Sprint(out["provider_payment_id"]))
	status:=strings.ToUpper(strings.TrimSpace(fmt.Sprint(out["status"])))
	if status==""{status="PROCESSING"}
	_,_=a.db.ExecContext(ctx,`UPDATE billing.invoices SET provider_status=$2,payment_attempt_id=$3,provider_payment_id=$4 WHERE id=$1 AND status<>'PAID'`,
		invoiceID,status,attemptID,providerID)
}
