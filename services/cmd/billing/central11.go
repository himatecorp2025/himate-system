package main

import "himate.local/services/internal/common"

// central11BillingTestPurgeMigration preserves the production immutability
// guarantees while allowing one transaction-scoped, explicitly targeted
// Golden Test Partner purge. The bypass is effective only when the current
// transaction sets himate.test_partner_purge to the exact OLD.partner_id.
func central11BillingTestPurgeMigration() common.Migration {
	return common.Migration{
		Version: 20,
		Name:    "central-11-golden-test-partner-purge-guards",
		Statements: []string{
			`CREATE OR REPLACE FUNCTION billing.reject_module_snapshot_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				IF TG_OP='DELETE' AND current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id THEN
					RETURN OLD;
				END IF;
				RAISE EXCEPTION 'billing.module_period_snapshots is immutable';
				RETURN OLD;
			END; $fn$`,
			`CREATE OR REPLACE FUNCTION billing.reject_billing_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				IF TG_OP='DELETE' AND current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id THEN
					RETURN OLD;
				END IF;
				RAISE EXCEPTION 'billing.billing_events is append-only';
				RETURN OLD;
			END; $fn$`,
			`CREATE OR REPLACE FUNCTION billing.guard_invoice_item_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				IF TG_OP='DELETE' THEN
					IF current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id THEN
						RETURN OLD;
					END IF;
					RAISE EXCEPTION 'billing.invoice_items is append-only';
				END IF;
				IF OLD.item_key IS DISTINCT FROM NEW.item_key
					OR OLD.partner_id IS DISTINCT FROM NEW.partner_id
					OR OLD.module_key IS DISTINCT FROM NEW.module_key
					OR OLD.item_type IS DISTINCT FROM NEW.item_type
					OR OLD.description IS DISTINCT FROM NEW.description
					OR OLD.currency IS DISTINCT FROM NEW.currency
					OR OLD.quantity IS DISTINCT FROM NEW.quantity
					OR OLD.unit_price IS DISTINCT FROM NEW.unit_price
					OR OLD.amount IS DISTINCT FROM NEW.amount
					OR OLD.period_start IS DISTINCT FROM NEW.period_start
					OR OLD.period_end IS DISTINCT FROM NEW.period_end
					OR OLD.snapshot_id IS DISTINCT FROM NEW.snapshot_id
					OR OLD.billing_model IS DISTINCT FROM NEW.billing_model
					OR OLD.created_at IS DISTINCT FROM NEW.created_at THEN
					RAISE EXCEPTION 'billing.invoice_items commercial fields are immutable';
				END IF;
				IF OLD.invoice_id IS NOT NULL AND OLD.invoice_id IS DISTINCT FROM NEW.invoice_id THEN
					RAISE EXCEPTION 'billing.invoice_items invoice assignment is immutable once set';
				END IF;
				IF OLD.status='INVOICED' AND NEW.status IS DISTINCT FROM OLD.status THEN
					RAISE EXCEPTION 'billing.invoice_items invoiced status cannot be reversed';
				END IF;
				IF OLD.invoiced_at IS NOT NULL AND OLD.invoiced_at IS DISTINCT FROM NEW.invoiced_at THEN
					RAISE EXCEPTION 'billing.invoice_items invoiced_at is immutable once set';
				END IF;
				RETURN NEW;
			END; $fn$`,
			`CREATE OR REPLACE FUNCTION billing.reject_partner_terms_history_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				IF TG_OP='DELETE' AND current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id THEN
					RETURN OLD;
				END IF;
				RAISE EXCEPTION 'billing.partner_terms_history is append-only';
				RETURN OLD;
			END; $fn$`,
			`CREATE OR REPLACE FUNCTION billing.guard_plan_invoice_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				IF TG_OP='DELETE' THEN
					IF current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id THEN
						RETURN OLD;
					END IF;
					IF OLD.billing_model='PLAN' THEN
						RAISE EXCEPTION 'PLAN invoices are append-only';
					END IF;
					RETURN OLD;
				END IF;
				IF OLD.billing_model IS DISTINCT FROM NEW.billing_model THEN
					RAISE EXCEPTION 'invoice billing_model is immutable';
				END IF;
				IF OLD.billing_model='PLAN' AND (
					OLD.invoice_key IS DISTINCT FROM NEW.invoice_key
					OR OLD.partner_id IS DISTINCT FROM NEW.partner_id
					OR OLD.invoice_date IS DISTINCT FROM NEW.invoice_date
					OR OLD.service_period_start IS DISTINCT FROM NEW.service_period_start
					OR OLD.service_period_end IS DISTINCT FROM NEW.service_period_end
					OR OLD.currency IS DISTINCT FROM NEW.currency
					OR OLD.base_fee IS DISTINCT FROM NEW.base_fee
					OR OLD.module_fee IS DISTINCT FROM NEW.module_fee
					OR OLD.total IS DISTINCT FROM NEW.total
					OR OLD.net_total IS DISTINCT FROM NEW.net_total
					OR OLD.tax_rate_percent IS DISTINCT FROM NEW.tax_rate_percent
					OR OLD.tax_amount IS DISTINCT FROM NEW.tax_amount
					OR OLD.minimum_commitment_adjustment IS DISTINCT FROM NEW.minimum_commitment_adjustment
					OR OLD.plan_key IS DISTINCT FROM NEW.plan_key
					OR OLD.billing_frequency IS DISTINCT FROM NEW.billing_frequency
					OR OLD.charge_type IS DISTINCT FROM NEW.charge_type
					OR OLD.list_price IS DISTINCT FROM NEW.list_price
					OR OLD.discount_amount IS DISTINCT FROM NEW.discount_amount
					OR OLD.created_at IS DISTINCT FROM NEW.created_at
				) THEN
					RAISE EXCEPTION 'PLAN invoice commercial fields are immutable';
				END IF;
				RETURN NEW;
			END; $fn$`,
		},
	}
}
