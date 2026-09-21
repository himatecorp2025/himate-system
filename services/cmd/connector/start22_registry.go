package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	start22ProtocolVersion = "1.0"
	start22SourceSystem     = "KLAVIERHAUS"
	start22RetentionYears  = 7
)

type start22DatasetDefinition struct {
	ModuleKey             string   `json:"module_key"`
	DatasetKey            string   `json:"dataset_key"`
	TransferMode          string   `json:"transfer_mode"`
	TargetService         string   `json:"target_service"`
	Cadence               string   `json:"cadence"`
	ContainsPersonalData  bool     `json:"contains_personal_data"`
	ContainsSensitiveData bool     `json:"contains_sensitive_data"`
	AllowedFields         []string `json:"allowed_fields"`
}

var start22DatasetRegistry = []start22DatasetDefinition{
	{ModuleKey:"finance",DatasetKey:"finance.balance_sheet",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",AllowedFields:[]string{"assets_usd","liabilities_usd","equity_usd","account_count"}},
	{ModuleKey:"income_statement",DatasetKey:"finance.income_statement",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",AllowedFields:[]string{"revenue_usd","expense_usd","net_income_usd","financial_item_count"}},
	{ModuleKey:"invoice_documents",DatasetKey:"finance.invoices",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"EVENT_AND_DAILY",ContainsPersonalData:true,AllowedFields:[]string{"invoice_count","paid_count","overdue_count","invoiced_usd","paid_usd","outstanding_usd"}},
	{ModuleKey:"audit_log",DatasetKey:"operations.audit",TransferMode:"AGGREGATE",TargetService:"connector",Cadence:"HOURLY",ContainsPersonalData:true,AllowedFields:[]string{"event_count","failed_count","financial_event_count","work_event_count"}},
	{ModuleKey:"backups",DatasetKey:"operations.backups",TransferMode:"METADATA",TargetService:"health",Cadence:"FIVE_MINUTES",AllowedFields:[]string{"backup_count","successful_count","failed_count","latest_age_hours","latest_size_bytes"}},
	{ModuleKey:"pianos",DatasetKey:"operations.client_pianos",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"piano_count","verified_count","located_count"}},
	{ModuleKey:"contacts",DatasetKey:"crm.clients",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,ContainsSensitiveData:true,AllowedFields:[]string{"client_count","active_count","vip_count","buying_interest_count"}},
	{ModuleKey:"closed_jobs",DatasetKey:"operations.closed_jobs",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"closed_count","billed_usd","average_cycle_hours","paid_count"}},
	{ModuleKey:"knowledge_base",DatasetKey:"evidence.company_documents",TransferMode:"METADATA",TargetService:"evidence",Cadence:"EVENT_AND_DAILY",ContainsPersonalData:true,AllowedFields:[]string{"document_count","financial_document_count","stored_file_count","total_amount_usd"}},
	{ModuleKey:"company_data",DatasetKey:"partner.company_profile",TransferMode:"METADATA",TargetService:"partners",Cadence:"ON_CHANGE",ContainsPersonalData:true,AllowedFields:[]string{"legal_name","trade_name","city","state","country","business_email","business_phone","profile_completeness_rate"}},
	{ModuleKey:"inventory",DatasetKey:"operations.inventory",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"item_count","quantity_total","reserved_quantity","inventory_value_usd"}},
	{ModuleKey:"partners",DatasetKey:"crm.business_partners",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"partner_count","active_count","contractor_count"}},
	{ModuleKey:"planned_jobs",DatasetKey:"operations.planned_jobs",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",ContainsPersonalData:true,AllowedFields:[]string{"planned_count","expected_revenue_usd","weighted_revenue_usd","estimated_hours"}},
	{ModuleKey:"scheduler",DatasetKey:"operations.scheduler",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",ContainsPersonalData:true,AllowedFields:[]string{"scheduled_job_count","planned_hours","active_assignee_count","overdue_job_count"}},
	{ModuleKey:"website_services",DatasetKey:"catalog.services",TransferMode:"METADATA",TargetService:"partners",Cadence:"ON_CHANGE",AllowedFields:[]string{"service_count","visible_count","featured_count"}},
	{ModuleKey:"settings",DatasetKey:"system.settings",TransferMode:"METADATA",TargetService:"connector",Cadence:"ON_CHANGE",AllowedFields:[]string{"enabled_feature_count","configured_setting_count"}},
	{ModuleKey:"system_integrations",DatasetKey:"system.integrations",TransferMode:"METADATA",TargetService:"health",Cadence:"FIVE_MINUTES",ContainsSensitiveData:true,AllowedFields:[]string{"integration_count","enabled_count","healthy_count","error_count"}},
	{ModuleKey:"users",DatasetKey:"operations.users",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,ContainsSensitiveData:true,AllowedFields:[]string{"active_user_count","admin_count","manager_count","worker_count"}},
	{ModuleKey:"workshop_workflow",DatasetKey:"operations.workshop",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",ContainsPersonalData:true,AllowedFields:[]string{"active_job_count","backlog_count","completed_count","average_cycle_hours"}},
	{ModuleKey:"marketing_overview",DatasetKey:"marketing.overview",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",AllowedFields:[]string{"session_count","lead_count","conversion_count","conversion_rate"}},
	{ModuleKey:"customer_inbox",DatasetKey:"marketing.customer_inbox",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",ContainsPersonalData:true,ContainsSensitiveData:true,AllowedFields:[]string{"conversation_count","open_count","closed_count","average_response_minutes"}},
	{ModuleKey:"website_reviews",DatasetKey:"marketing.reviews",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"review_count","published_count","event_linked_count"}},
	{ModuleKey:"campaigns_utm",DatasetKey:"marketing.campaigns",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",AllowedFields:[]string{"campaign_count","active_count","tracked_session_count","conversion_count"}},
	{ModuleKey:"leads",DatasetKey:"marketing.leads",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",ContainsPersonalData:true,ContainsSensitiveData:true,AllowedFields:[]string{"lead_count","contacted_count","appointment_count","closed_count","conversion_rate"}},
	{ModuleKey:"tracking_cookies",DatasetKey:"marketing.tracking",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",ContainsPersonalData:true,AllowedFields:[]string{"session_count","event_count","analytics_consent_count","marketing_consent_count"}},
	{ModuleKey:"seo_keywords",DatasetKey:"marketing.seo",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",AllowedFields:[]string{"page_count","published_page_count","version_count","keyword_count"}},
	{ModuleKey:"heatmap",DatasetKey:"marketing.heatmap",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"consented_session_count","interaction_count","tracked_path_count"}},
	{ModuleKey:"website_artists",DatasetKey:"website.artists",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"artist_count","published_count","featured_count"}},
	{ModuleKey:"website_contacts",DatasetKey:"website.contacts",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"HOURLY",ContainsPersonalData:true,ContainsSensitiveData:true,AllowedFields:[]string{"contact_request_count","contacted_count","appointment_count","closed_count"}},
	{ModuleKey:"digital_attendance",DatasetKey:"events.attendance",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"EVENT_AND_DAILY",ContainsPersonalData:true,AllowedFields:[]string{"attendee_count","checkin_count","deleted_entry_count","attendance_rate"}},
	{ModuleKey:"events",DatasetKey:"events.events",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"EVENT_AND_DAILY",AllowedFields:[]string{"event_count","published_count","completed_count","capacity_total","ticket_revenue_usd"}},
	{ModuleKey:"event_guest_list",DatasetKey:"events.guests",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"EVENT_AND_DAILY",ContainsPersonalData:true,AllowedFields:[]string{"guest_count","confirmed_count","declined_count"}},
	{ModuleKey:"event_invitations",DatasetKey:"events.invitations",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"EVENT_AND_DAILY",ContainsPersonalData:true,ContainsSensitiveData:true,AllowedFields:[]string{"sent_count","accepted_count","declined_count","delivery_failed_count","rsvp_rate"}},
	{ModuleKey:"media_library",DatasetKey:"website.media",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",ContainsPersonalData:true,AllowedFields:[]string{"asset_count","image_count","video_count","storage_bytes"}},
	{ModuleKey:"pages_content",DatasetKey:"website.pages",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"ON_CHANGE",AllowedFields:[]string{"page_count","published_count","version_count","language_variant_count"}},
	{ModuleKey:"publish_preview",DatasetKey:"website.publish",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"ON_CHANGE",ContainsPersonalData:true,AllowedFields:[]string{"publish_count","preview_version_count","draft_version_count"}},
	{ModuleKey:"showroom_pianos",DatasetKey:"website.showroom_pianos",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"DAILY",AllowedFields:[]string{"piano_count","published_count","available_count","featured_count"}},
	{ModuleKey:"event_tickets",DatasetKey:"events.tickets",TransferMode:"AGGREGATE",TargetService:"impact",Cadence:"EVENT_AND_DAILY",ContainsPersonalData:true,ContainsSensitiveData:true,AllowedFields:[]string{"ticket_count","paid_count","refunded_count","checked_in_count","gross_usd","refund_usd"}},
}

var start22DatasetByKey = func() map[string]start22DatasetDefinition {
	out := make(map[string]start22DatasetDefinition, len(start22DatasetRegistry))
	for _, definition := range start22DatasetRegistry {
		out[definition.DatasetKey] = definition
	}
	return out
}()

func start22ValidateRegistry() error {
	if len(start22DatasetRegistry) != 38 {
		return fmt.Errorf("START-22 registry must contain exactly 38 module datasets, got %d", len(start22DatasetRegistry))
	}
	modules := map[string]bool{}
	datasets := map[string]bool{}
	for _, definition := range start22DatasetRegistry {
		if strings.TrimSpace(definition.ModuleKey) == "" || strings.TrimSpace(definition.DatasetKey) == "" {
			return fmt.Errorf("START-22 registry contains an empty module or dataset key")
		}
		if modules[definition.ModuleKey] {
			return fmt.Errorf("duplicate START-22 module_key %s", definition.ModuleKey)
		}
		if datasets[definition.DatasetKey] {
			return fmt.Errorf("duplicate START-22 dataset_key %s", definition.DatasetKey)
		}
		if len(definition.AllowedFields) == 0 {
			return fmt.Errorf("dataset %s has no allowed fields", definition.DatasetKey)
		}
		modules[definition.ModuleKey] = true
		datasets[definition.DatasetKey] = true
	}
	return nil
}

func start22RegistryPayload() []map[string]any {
	items := make([]map[string]any, 0, len(start22DatasetRegistry))
	for _, definition := range start22DatasetRegistry {
		fields := append([]string(nil), definition.AllowedFields...)
		sort.Strings(fields)
		items = append(items, map[string]any{
			"module_key": definition.ModuleKey,
			"dataset_key": definition.DatasetKey,
			"transfer_mode": definition.TransferMode,
			"target_service": definition.TargetService,
			"cadence": definition.Cadence,
			"contains_personal_data": definition.ContainsPersonalData,
			"contains_sensitive_data": definition.ContainsSensitiveData,
			"allowed_fields": fields,
			"retention_policy": "HIMATE_7Y",
			"retention_years": start22RetentionYears,
			"schema_version": 1,
		})
	}
	return items
}
