package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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


func TestPartnerModuleEntitledRequiresActiveExecutableInvoiceModule(t *testing.T) {
	tests := []struct {
		name        string
		accessState string
		executable  bool
		want        bool
	}{
		{name: "active executable", accessState: "ACTIVE", executable: true, want: true},
		{name: "locked", accessState: "LOCKED", executable: true, want: false},
		{name: "not executable", accessState: "ACTIVE", executable: false, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/internal/v1/partner-portal/ptr_1/modules" {
					http.Error(w, "unexpected path", http.StatusNotFound)
					return
				}
				if r.Header.Get("X-Himate-Internal-Token") == "" {
					http.Error(w, "internal token missing", http.StatusForbidden)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{
					{"key": invoiceModuleKey, "access_state": tc.accessState, "executable": tc.executable},
				}})
			}))
			defer server.Close()
			a := &app{
				catalogHost:   strings.TrimPrefix(server.URL, "http://"),
				internalToken: "0123456789abcdef0123456789abcdef",
				client:        server.Client(),
			}
			got, err := a.partnerModuleEntitled(context.Background(), "ptr_1", invoiceModuleKey)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("entitled=%v want %v", got, tc.want)
			}
		})
	}
}
