package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"himate.local/services/internal/automation"
	"himate.local/services/internal/common"
)

const (
	workflowInvoiceReadyEvent = "workflow.billing_approved.v1"
	scheduleInvoiceReadyEvent = "scheduler.job_closed_invoice_ready.v1"
)

type automationInvoicePayload struct {
	Currency         string             `json:"currency"`
	Customer         customerSnapshot   `json:"customer"`
	Items            []invoiceItemInput `json:"items"`
	PaymentTermsDays *int               `json:"payment_terms_days,omitempty"`
	Notes            string             `json:"notes,omitempty"`
}

type automationClaimResponse struct {
	Items []automationDelivery `json:"items"`
}

type automationDelivery struct {
	DeliveryID  int64           `json:"delivery_id"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	Event       automationEvent `json:"event"`
}

type automationEvent struct {
	ID              int64           `json:"id"`
	EventKey        string          `json:"event_key"`
	EventType       string          `json:"event_type"`
	EventVersion    int             `json:"event_version"`
	ProducerService string          `json:"producer_service"`
	PartnerID       string          `json:"partner_id"`
	ModuleKey       string          `json:"module_key"`
	CorrelationID   string          `json:"correlation_id"`
	CausationID     string          `json:"causation_id"`
	SubjectType     string          `json:"subject_type"`
	SubjectID       string          `json:"subject_id"`
	Payload         json.RawMessage `json:"payload"`
}

func (a *app) automationCall(ctx context.Context, method, path string, payload any, out any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+a.automationHost+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	common.BindInternalRequest(req, a.internalToken)
	if err := automation.SignRequest(req, body, financeProducer, a.automationKey, time.Now().UTC()); err != nil {
		return err
	}
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("automation %s returned status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) ensureAutomationSubscriptions(ctx context.Context) error {
	payload := map[string]any{
		"event_types": []string{workflowInvoiceReadyEvent, scheduleInvoiceReadyEvent},
		"max_attempts": 8,
	}
	var lastErr error
	for {
		var out map[string]any
		lastErr = a.automationCall(ctx, http.MethodPost, "/internal/v1/automation/subscriptions", payload, &out)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("register tenant-finance subscriptions: %w", lastErr)
			}
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func decodeAutomationInvoicePayload(raw json.RawMessage) (automationInvoicePayload, error) {
	var payload automationInvoicePayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("invoice automation payload: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return payload, errors.New("invoice automation payload must contain exactly one JSON object")
	}
	return payload, nil
}

func intentFromAutomationDelivery(d automationDelivery) (automatedInvoiceIntent, error) {
	event := d.Event
	event.EventType = strings.TrimSpace(event.EventType)
	event.ProducerService = strings.TrimSpace(event.ProducerService)
	event.PartnerID = strings.TrimSpace(event.PartnerID)
	event.ModuleKey = strings.TrimSpace(event.ModuleKey)
	event.SubjectType = strings.TrimSpace(event.SubjectType)
	event.SubjectID = strings.TrimSpace(event.SubjectID)

	sourceType := ""
	switch event.EventType {
	case workflowInvoiceReadyEvent:
		if event.ProducerService != "workshop" {
			return automatedInvoiceIntent{}, errors.New("workflow billing event producer must be workshop")
		}
		if event.ModuleKey != "workshop_workflow" || event.SubjectType != "workflow" {
			return automatedInvoiceIntent{}, errors.New("workflow billing event has invalid module or subject type")
		}
		sourceType = sourceWorkflow
	case scheduleInvoiceReadyEvent:
		if event.ProducerService != "scheduler" {
			return automatedInvoiceIntent{}, errors.New("schedule billing event producer must be scheduler")
		}
		if event.ModuleKey != "scheduler" || event.SubjectType != "scheduled_job" {
			return automatedInvoiceIntent{}, errors.New("schedule billing event has invalid module or subject type")
		}
		sourceType = sourceSchedule
	default:
		return automatedInvoiceIntent{}, errors.New("unsupported tenant invoice automation event type")
	}
	if event.PartnerID == "" || event.SubjectID == "" {
		return automatedInvoiceIntent{}, errors.New("automation event partner_id and subject_id are required")
	}
	payload, err := decodeAutomationInvoicePayload(event.Payload)
	if err != nil {
		return automatedInvoiceIntent{}, err
	}
	return automatedInvoiceIntent{
		PartnerID:        event.PartnerID,
		SourceType:       sourceType,
		SourceID:         event.SubjectID,
		Currency:         payload.Currency,
		Customer:         payload.Customer,
		Items:            payload.Items,
		PaymentTermsDays: payload.PaymentTermsDays,
		Notes:            payload.Notes,
		CorrelationID:    event.CorrelationID,
		CausationID:      event.EventKey,
	}, nil
}

func (a *app) runAutomationConsumer(ctx context.Context, log interface{ Error(string, ...any) }) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := a.consumeAutomationOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("tenant finance automation consumer", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *app) consumeAutomationOnce(ctx context.Context) error {
	var claimed automationClaimResponse
	if err := a.automationCall(ctx, http.MethodPost, "/internal/v1/automation/deliveries/claim", map[string]any{"limit": 25}, &claimed); err != nil {
		return err
	}
	for _, delivery := range claimed.Items {
		if err := a.consumeInvoiceDelivery(ctx, delivery); err != nil {
			_ = a.failAutomationDelivery(context.Background(), delivery.DeliveryID, err)
			continue
		}
		if err := a.ackAutomationDelivery(context.Background(), delivery.DeliveryID); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) consumeInvoiceDelivery(ctx context.Context, delivery automationDelivery) error {
	intent, err := intentFromAutomationDelivery(delivery)
	if err != nil {
		return err
	}
	_, _, err = a.createAutomatedReady(
		ctx,
		strings.TrimSpace(delivery.Event.ProducerService),
		"service:"+strings.TrimSpace(delivery.Event.ProducerService),
		intent,
	)
	return err
}

func (a *app) ackAutomationDelivery(ctx context.Context, deliveryID int64) error {
	path := fmt.Sprintf("/internal/v1/automation/deliveries/%d/ack", deliveryID)
	return a.automationCall(ctx, http.MethodPost, path, nil, nil)
}

func (a *app) failAutomationDelivery(ctx context.Context, deliveryID int64, cause error) error {
	message := "tenant invoice automation failed"
	if cause != nil {
		message = strings.TrimSpace(cause.Error())
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	path := fmt.Sprintf("/internal/v1/automation/deliveries/%d/fail", deliveryID)
	return a.automationCall(ctx, http.MethodPost, path, map[string]any{
		"error": message,
		"retry_after_seconds": 15,
	}, nil)
}
