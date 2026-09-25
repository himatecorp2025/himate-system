package main

import (
	"database/sql"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"strings"
	"time"
)

type portalModule struct {
	Key                  string
	Label                string
	Description          string
	MarketplaceSummary   string
	GroupKey             string
	GroupLabel           string
	Status               string
	EntitlementState     string
	IncludedInBase       bool
	CommercialConfigured bool
	CommercialReady      bool
	QuoteReference       string
	PartnerPrice         float64
	Currency             string
	LatestVersion        string
	Availability         string
	PublicationStatus    string
	ImplementationState  string
	MarketplaceVisible   bool
	Executable           bool
	AccessState          string
	ActivatedAt          any
	Relationships        []map[string]any
	CanActivate          bool
	Blockers              []string
}

func (a *app) partnerPortal(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/partner-portal/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "modules" {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner portal catalog route not found")
		return
	}
	partnerID := parts[0]
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
			return
		}
		a.partnerPortalModules(w, r, partnerID)
		return
	}
	if len(parts) == 4 && parts[3] == "activate" {
		if r.Method != http.MethodPost {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
			return
		}
		a.partnerPortalActivate(w, r, partnerID, parts[2])
		return
	}
	common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner portal catalog route not found")
}

func (a *app) loadPartnerPortalModules(partnerID, locale string) ([]portalModule, error) {
	if err := a.ensurePartnerModules(partnerID); err != nil {
		return nil, err
	}
	var testPartner bool
	if err := a.db.QueryRow(`SELECT test_partner FROM partners.partners WHERE id=$1`, partnerID).Scan(&testPartner); err != nil {
		return nil, err
	}
	visibilityClause := " AND (m.marketplace_visible=TRUE OR m.publication_status='PUBLISHED')"
	if testPartner {
		visibilityClause = " AND m.system=TRUE"
	}
	rows, err := a.db.Query(`
		SELECT m.module_key,m.label_en,m.label_hu,m.description_en,m.description_hu,m.marketplace_summary_en,m.marketplace_summary_hu,
			m.group_key,g.label_en,g.label_hu,pm.status,pm.entitlement_state,pm.included_in_base,
			pm.commercial_configured,
			(pm.commercial_configured=TRUE AND (pm.included_in_base=TRUE OR pm.price_override IS NOT NULL OR ep.new_price IS NOT NULL)),
			pm.quote_reference,
			CASE WHEN pm.commercial_configured THEN COALESCE(ep.new_price,pm.price_override,0) ELSE 0 END,
			COALESCE(NULLIF(pm.contract_currency,''),m.currency),m.latest_version,m.availability,m.publication_status,m.implementation_state,m.marketplace_visible,pm.activated_at
		FROM catalog.partner_modules pm
		JOIN catalog.modules m ON m.module_key=pm.module_key
		JOIN catalog.module_groups g ON g.group_key=m.group_key
		LEFT JOIN LATERAL (
			SELECT ph.new_price FROM catalog.price_history ph
			WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at<=NOW()
			ORDER BY ph.effective_at DESC,ph.id DESC LIMIT 1
		) ep ON TRUE
		WHERE pm.partner_id=$1`+visibilityClause+`
		ORDER BY g.sort_order,m.label_en`, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modules := []portalModule{}
	active := map[string]bool{}
	index := map[string]int{}
	for rows.Next() {
		var item portalModule
		var labelEN,labelHU,descEN,descHU,summaryEN,summaryHU,groupEN,groupHU string
		var activated sql.NullTime
		if err := rows.Scan(&item.Key,&labelEN,&labelHU,&descEN,&descHU,&summaryEN,&summaryHU,&item.GroupKey,&groupEN,&groupHU,&item.Status,&item.EntitlementState,&item.IncludedInBase,
			&item.CommercialConfigured,&item.CommercialReady,&item.QuoteReference,&item.PartnerPrice,&item.Currency,&item.LatestVersion,&item.Availability,
			&item.PublicationStatus,&item.ImplementationState,&item.MarketplaceVisible,&activated); err != nil {
			return nil, err
		}
		item.Label = common.Localized(labelEN,labelHU,locale)
		item.Description = common.Localized(descEN,descHU,locale)
		item.MarketplaceSummary = common.Localized(summaryEN,summaryHU,locale)
		if strings.TrimSpace(item.MarketplaceSummary) == "" {
			item.MarketplaceSummary = item.Description
		}
		item.GroupLabel = common.Localized(groupEN,groupHU,locale)
		item.Executable = marketplaceExecutable(item.PublicationStatus,item.ImplementationState,item.Availability)
		item.AccessState = marketplaceAccessState(item.Status,item.EntitlementState,item.PublicationStatus,item.ImplementationState,item.Availability)
		if testPartner && !strings.EqualFold(strings.TrimSpace(item.ImplementationState), "IN_DEVELOPMENT") {
			item.Executable = true
			item.AccessState = "ACTIVE"
			item.CommercialConfigured = true
			item.CommercialReady = true
		}
		if activated.Valid { item.ActivatedAt = activated.Time.UTC() }
		item.Relationships = []map[string]any{}
		item.Blockers = []string{}
		active[item.Key] = item.Status == "ACTIVE"
		index[item.Key] = len(modules)
		modules = append(modules, item)
	}
	if err := rows.Err(); err != nil { return nil, err }

	relRows, err := a.db.Query(`
		SELECT r.module_key,r.target_module_key,m.label_en,m.label_hu,r.relation_type,r.note
		FROM catalog.module_relationships r
		JOIN catalog.modules m ON m.module_key=r.target_module_key
		WHERE r.relation_type IN ('REQUIRES','OPTIONAL_DEPENDENCY','INTEGRATES_WITH','CONFLICTS_WITH','REPLACES')
		ORDER BY r.module_key,r.relation_type,m.label_en`)
	if err != nil { return nil, err }
	defer relRows.Close()
	for relRows.Next() {
		var key,target,labelEN,labelHU,relation,note string
		if relRows.Scan(&key,&target,&labelEN,&labelHU,&relation,&note) != nil { continue }
		label:=common.Localized(labelEN,labelHU,locale)
		i, ok := index[key]
		if !ok { continue }
		modules[i].Relationships = append(modules[i].Relationships, map[string]any{
			"target_module_key": target, "target_label": label, "target_label_en": labelEN, "target_label_hu": labelHU,
			"relation_type": relation, "note": note, "target_active": active[target],
		})
		if relation == "REQUIRES" && !active[target] {
			modules[i].Blockers = append(modules[i].Blockers, "Requires "+label)
		}
		if relation == "CONFLICTS_WITH" && active[target] {
			modules[i].Blockers = append(modules[i].Blockers, "Conflicts with active "+label)
		}
	}
	if err := relRows.Err(); err != nil { return nil, err }

	for i := range modules {
		if testPartner {
			modules[i].Blockers = []string{}
			modules[i].CanActivate = false
			continue
		}
		if modules[i].PublicationStatus != "PUBLISHED" {
			modules[i].Blockers = append(modules[i].Blockers, "Module is visible in the marketplace but is not published for live use yet")
		}
		if modules[i].ImplementationState != "READY" {
			modules[i].Blockers = append(modules[i].Blockers, "Module reconstruction is not complete")
		}
		if modules[i].Availability != "ACTIVE" {
			modules[i].Blockers = append(modules[i].Blockers, "Module is not currently available")
		}
		if modules[i].Status == "MAINTENANCE" {
			modules[i].Blockers = append(modules[i].Blockers, "Module is restricted by HIMATE maintenance")
		}
		if !modules[i].CommercialReady {
			modules[i].Blockers = append(modules[i].Blockers, "Partner-specific commercial terms are not ready")
		}
		modules[i].CanActivate = modules[i].Executable && modules[i].Status != "ACTIVE" && len(modules[i].Blockers) == 0
	}
	return modules, nil
}

func (a *app) partnerPortalModules(w http.ResponseWriter, r *http.Request, partnerID string) {
	modules, err := a.loadPartnerPortalModules(partnerID, common.RequestLocale(r))
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load partner portal modules")
		return
	}
	items := make([]map[string]any, 0, len(modules))
	activeCount := 0
	availableCount := 0
	lockedCount := 0
	comingSoonCount := 0
	for _, item := range modules {
		switch item.AccessState {
		case "ACTIVE":
			activeCount++
		case "LOCKED":
			lockedCount++
		case "COMING_SOON":
			comingSoonCount++
		}
		if item.CanActivate { availableCount++ }
		items = append(items, portalModuleMap(item))
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"partner_id": partnerID, "items": items, "count": len(items),
		"active_count": activeCount, "available_count": availableCount,
		"locked_count": lockedCount, "coming_soon_count": comingSoonCount,
		"marketplace_model": "DISCOVERY_SEPARATE_FROM_EXECUTION",
		"locale": common.RequestLocale(r),
	})
}


func portalModuleMap(item portalModule) map[string]any {
	return map[string]any{
		"key": item.Key, "label": item.Label, "description": item.Description, "marketplace_summary": item.MarketplaceSummary,
		"group_key": item.GroupKey, "group_label": item.GroupLabel,
		"status": item.Status, "entitlement_state": item.EntitlementState, "included_in_base": item.IncludedInBase,
		"commercial_configured":item.CommercialConfigured,"commercial_ready":item.CommercialReady,"quote_reference":item.QuoteReference,
		"partner_price": item.PartnerPrice, "currency": item.Currency,
		"latest_version": item.LatestVersion, "availability": item.Availability,
		"publication_status":item.PublicationStatus,"implementation_state":item.ImplementationState,
		"marketplace_visible":item.MarketplaceVisible,"executable":item.Executable,"access_state":item.AccessState,
		"activated_at": item.ActivatedAt, "relationships": item.Relationships,
		"can_activate": item.CanActivate, "activation_blockers": item.Blockers,
	}
}

func (a *app) partnerPortalActivate(w http.ResponseWriter, r *http.Request, partnerID, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Module not found")
		return
	}
	if err := a.ensurePartnerModules(partnerID); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not initialize partner module state")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not start module activation")
		return
	}
	defer tx.Rollback()

	var status, availability, publicationStatus, labelEN, labelHU string
	var visible,commercialConfigured,commercialReady bool
	if err := tx.QueryRow(`
		SELECT pm.status,m.availability,m.publication_status,m.label_en,m.label_hu,pm.visible,pm.commercial_configured,
			(pm.commercial_configured=TRUE AND (pm.included_in_base=TRUE OR pm.price_override IS NOT NULL OR EXISTS (
				SELECT 1 FROM catalog.price_history ph
				WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at<=NOW()
			)))
		FROM catalog.partner_modules pm
		JOIN catalog.modules m ON m.module_key=pm.module_key
		WHERE pm.partner_id=$1 AND pm.module_key=$2
		FOR UPDATE`, partnerID, key).Scan(&status,&availability,&publicationStatus,&labelEN,&labelHU,&visible,&commercialConfigured,&commercialReady); err != nil {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Module not found")
		return
	}
	if publicationStatus != "PUBLISHED" {
		common.APIError(w,http.StatusConflict,"MODULE_UNPUBLISHED","Module is not published for partner use")
		return
	}
	if !commercialConfigured || !commercialReady {
		common.APIError(w,http.StatusConflict,"COMMERCIAL_TERMS_REQUIRED","Partner-specific module commercial terms must include an explicit partner price or base-package inclusion before activation")
		return
	}
	if status == "ACTIVE" {
		tx.Rollback()
		a.onePartnerModule(w, partnerID, key, common.RequestLocale(r))
		return
	}
	if status == "MAINTENANCE" {
		common.APIError(w, http.StatusConflict, "MODULE_MAINTENANCE", "Module is restricted by HIMATE maintenance")
		return
	}
	if availability != "ACTIVE" {
		common.APIError(w, http.StatusConflict, "MODULE_UNAVAILABLE", "Module is not currently available")
		return
	}

	rows, err := tx.Query(`
		SELECT r.target_module_key,m.label_en,m.label_hu,r.relation_type,pm.status
		FROM catalog.module_relationships r
		JOIN catalog.modules m ON m.module_key=r.target_module_key
		LEFT JOIN catalog.partner_modules pm ON pm.partner_id=$1 AND pm.module_key=r.target_module_key
		WHERE r.module_key=$2 AND r.relation_type IN ('REQUIRES','CONFLICTS_WITH')`, partnerID, key)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not validate module relationships")
		return
	}
	blockers := []string{}
	for rows.Next() {
		var target,targetLabelEN,targetLabelHU,relation string
		var targetStatus sql.NullString
		if rows.Scan(&target,&targetLabelEN,&targetLabelHU,&relation,&targetStatus) != nil { continue }
		targetLabel:=common.Localized(targetLabelEN,targetLabelHU,common.RequestLocale(r))
		isActive := targetStatus.Valid && targetStatus.String == "ACTIVE"
		if relation == "REQUIRES" && !isActive {
			blockers = append(blockers, "Requires "+targetLabel)
		}
		if relation == "CONFLICTS_WITH" && isActive {
			blockers = append(blockers, "Conflicts with active "+targetLabel)
		}
		_ = target
	}
	rows.Close()
	if len(blockers) > 0 {
		common.APIError(w, http.StatusConflict, "MODULE_DEPENDENCY_BLOCKED", strings.Join(blockers, "; "))
		return
	}

	now := time.Now().UTC()
	if _, err = tx.Exec(`UPDATE catalog.partner_modules SET status='ACTIVE',entitlement_state='ACTIVE',visible=TRUE,activated_at=$3,updated_at=NOW()
		WHERE partner_id=$1 AND module_key=$2`, partnerID, key, now); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not activate module")
		return
	}
	actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if actor == "" { actor = "partner-portal" }
	reason := "Partner Portal activation"
	if _, err = tx.Exec(`INSERT INTO catalog.partner_module_history(partner_id,module_key,field_name,old_value,new_value,effective_at,actor,reason)
		VALUES($1,$2,'status',$3,'ACTIVE',$4,$5,$6)`,partnerID,key,status,now,actor,reason); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not record module activation history")
		return
	}
	if !visible {
		if _, err = tx.Exec(`INSERT INTO catalog.partner_module_history(partner_id,module_key,field_name,old_value,new_value,effective_at,actor,reason)
			VALUES($1,$2,'visible','false','true',$3,$4,$5)`,partnerID,key,now,actor,reason); err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not record module visibility history")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not commit module activation")
		return
	}

	item, err := scanPartnerModule(a.db.QueryRow(partnerModuleSelect+` WHERE pm.partner_id=$1 AND pm.module_key=$2`, partnerID, key), common.RequestLocale(r))
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Module activated but could not be reloaded")
		return
	}
	item["activation_message"] = fmt.Sprintf("%s activated", common.Localized(labelEN,labelHU,common.RequestLocale(r)))
	common.JSON(w, http.StatusOK, item)
}
