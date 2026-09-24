package main

import (
	"encoding/json"
	"testing"
)

func TestIntentFromAutomationDeliveryBindsTenantAndSourceToEnvelope(t *testing.T) {
	payload, _ := json.Marshal(automationInvoicePayload{
		Currency: "USD",
		Customer: customerSnapshot{
			DisplayName: "Customer",
			Country: "United States",
			StateRegion: "NY",
			City: "New York",
			PostalCode: "10001",
			AddressLine1: "1 Customer St",
		},
		Items: []invoiceItemInput{{
			Description: "Tuning",
			QuantityMilli: 1000,
			UnitPriceMinor: 10000,
		}},
	})
	delivery := automationDelivery{Event: automationEvent{
		EventKey: "scheduler-1",
		EventType: scheduleInvoiceReadyEvent,
		ProducerService: "scheduler",
		PartnerID: "ptr_authoritative",
		ModuleKey: "scheduler",
		SubjectType: "scheduled_job",
		SubjectID: "job_1",
		CorrelationID: "corr_1",
		Payload: payload,
	}}
	intent, err := intentFromAutomationDelivery(delivery)
	if err != nil {
		t.Fatal(err)
	}
	if intent.PartnerID != "ptr_authoritative" || intent.SourceType != sourceSchedule || intent.SourceID != "job_1" {
		t.Fatalf("envelope identity was not preserved: %+v", intent)
	}
}

func TestIntentFromAutomationDeliveryRejectsProducerImpersonation(t *testing.T) {
	payload := json.RawMessage(`{"currency":"USD","customer":{"display_name":"Customer"},"items":[{"description":"Tuning","quantity_milli":1000,"unit_price_minor":10000}]}`)
	_, err := intentFromAutomationDelivery(automationDelivery{Event: automationEvent{
		EventType: workflowInvoiceReadyEvent,
		ProducerService: "scheduler",
		PartnerID: "ptr_1",
		ModuleKey: "workshop_workflow",
		SubjectType: "workflow",
		SubjectID: "wf_1",
		Payload: payload,
	}})
	if err == nil {
		t.Fatal("scheduler must not be accepted as a Workshop billing producer")
	}
}

func TestAutomationPayloadCannotOverrideTenantOrSourceIdentity(t *testing.T) {
	raw := json.RawMessage(`{"partner_id":"ptr_attacker","currency":"USD","customer":{"display_name":"Customer"},"items":[{"description":"Tuning","quantity_milli":1000,"unit_price_minor":10000}]}`)
	if _, err := decodeAutomationInvoicePayload(raw); err == nil {
		t.Fatal("producer payload must not be able to override envelope partner_id")
	}
}
