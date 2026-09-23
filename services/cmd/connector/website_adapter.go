package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func start223ConnectorMigration() common.Migration {
	return common.Migration{
		Version: 6,
		Name: "start-22-3-website-adapter",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS connector.website_adapters(
				partner_id TEXT NOT NULL,
				environment TEXT NOT NULL,
				adapter_type TEXT NOT NULL DEFAULT 'GENERIC_HTTP',
				site_base_url TEXT NOT NULL DEFAULT '',
				allowed_domains JSONB NOT NULL DEFAULT '[]'::jsonb,
				capabilities JSONB NOT NULL DEFAULT '["ENTITLEMENTS","HEARTBEAT","METRICS","AGGREGATED_DATA","RECONCILIATION"]'::jsonb,
				privacy_mode TEXT NOT NULL DEFAULT 'AGGREGATED_ONLY',
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				config JSONB NOT NULL DEFAULT '{}'::jsonb,
				updated_by TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(partner_id,environment)
			)`,
			`CREATE INDEX IF NOT EXISTS connector_website_adapters_enabled_idx
				ON connector.website_adapters(enabled,environment,partner_id)`,
		},
	}
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok { return []string{} }
	out := make([]string,0,len(raw))
	for _, v := range raw {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" { out = append(out,s) }
	}
	return out
}

func normalizeDomain(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimPrefix(raw,"https://")
	raw = strings.TrimPrefix(raw,"http://")
	raw = strings.TrimSuffix(raw,"/")
	if i := strings.Index(raw,"/"); i >= 0 { raw = raw[:i] }
	if h, _, found := strings.Cut(raw,":"); found { raw = h }
	return raw
}

func validateAdapterURL(raw string) (string,error) {
	raw = strings.TrimSpace(raw)
	if raw == "" { return "",nil }
	u,err:=url.Parse(raw)
	if err!=nil || (u.Scheme!="https" && u.Scheme!="http") || strings.TrimSpace(u.Hostname())=="" {
		return "",fmt.Errorf("site_base_url must be an absolute HTTP(S) URL")
	}
	u.Fragment=""
	return strings.TrimRight(u.String(),"/"),nil
}

func validateAdapterDomains(baseURL string, domains []string) ([]string,error) {
	seen:=map[string]bool{}
	out:=[]string{}
	for _, raw:=range domains {
		d:=normalizeDomain(raw)
		if d=="" { continue }
		if strings.ContainsAny(d," /\\") { return nil,fmt.Errorf("invalid allowed domain") }
		if !seen[d] { seen[d]=true;out=append(out,d) }
	}
	if baseURL!="" {
		u,_:=url.Parse(baseURL)
		host:=strings.ToLower(u.Hostname())
		if host!=""&&!seen[host] {
			out=append(out,host)
			seen[host]=true
		}
	}
	if len(out)==0 { return nil,fmt.Errorf("at least one allowed domain is required") }
	return out,nil
}

func validAdapterCapabilities(values []string) ([]string,error) {
	allowed:=map[string]bool{
		"ENTITLEMENTS":true,"HEARTBEAT":true,"METRICS":true,
		"AGGREGATED_DATA":true,"RECONCILIATION":true,
	}
	seen:=map[string]bool{}
	out:=[]string{}
	for _, raw:=range values {
		v:=strings.ToUpper(strings.TrimSpace(raw))
		if !allowed[v] { return nil,fmt.Errorf("unsupported adapter capability %s",v) }
		if !seen[v] { seen[v]=true;out=append(out,v) }
	}
	if len(out)==0 { return nil,fmt.Errorf("at least one adapter capability is required") }
	return out,nil
}

func adapterSensitiveConfigKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, part := range []string{"password","secret","token","authorization","cookie","api_key","apikey","payment","invoice","bank","card"} {
		if strings.Contains(key, part) { return true }
	}
	return false
}

func validateAdapterConfigValue(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if adapterSensitiveConfigKey(key) {
				return fmt.Errorf("adapter config contains sensitive key %s", key)
			}
			if err := validateAdapterConfigValue(item); err != nil { return err }
		}
	case []any:
		for _, item := range typed {
			if err := validateAdapterConfigValue(item); err != nil { return err }
		}
	}
	return nil
}

func (a *app) websiteAdapterAdmin(w http.ResponseWriter,r *http.Request,partnerID string) {
	environment:=normalizeEnvironment(r.URL.Query().Get("environment"))
	if environment=="" { environment="PRODUCTION" }
	switch r.Method {
	case http.MethodGet:
		a.writeWebsiteAdapter(w,partnerID,environment)
	case http.MethodPut:
		var in struct{
			Environment string `json:"environment"`
			AdapterType string `json:"adapter_type"`
			SiteBaseURL string `json:"site_base_url"`
			AllowedDomains []string `json:"allowed_domains"`
			Capabilities []string `json:"capabilities"`
			Enabled *bool `json:"enabled"`
			Config map[string]any `json:"config"`
		}
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request");return }
		if env:=normalizeEnvironment(in.Environment);env!="" { environment=env }
		adapterType:=strings.ToUpper(strings.TrimSpace(in.AdapterType))
		if adapterType=="" { adapterType="GENERIC_HTTP" }
		if adapterType!="GENERIC_HTTP" && adapterType!="WORDPRESS" && adapterType!="CUSTOM_API" {
			common.APIError(w,400,"VALIDATION","adapter_type must be GENERIC_HTTP, WORDPRESS, or CUSTOM_API");return
		}
		baseURL,err:=validateAdapterURL(in.SiteBaseURL)
		if err!=nil { common.APIError(w,400,"VALIDATION",err.Error());return }
		domains,err:=validateAdapterDomains(baseURL,in.AllowedDomains)
		if err!=nil { common.APIError(w,400,"VALIDATION",err.Error());return }
		capabilities:=in.Capabilities
		if len(capabilities)==0 {
			capabilities=[]string{"ENTITLEMENTS","HEARTBEAT","METRICS","AGGREGATED_DATA","RECONCILIATION"}
		}
		capabilities,err=validAdapterCapabilities(capabilities)
		if err!=nil { common.APIError(w,400,"VALIDATION",err.Error());return }
		enabled:=true
		if in.Enabled!=nil { enabled=*in.Enabled }
		if in.Config==nil { in.Config=map[string]any{} }
		if err:=validateAdapterConfigValue(in.Config);err!=nil {
			common.APIError(w,400,"SENSITIVE_CONFIG",err.Error());return
		}
		domainsRaw,_:=json.Marshal(domains)
		capsRaw,_:=json.Marshal(capabilities)
		configRaw,_:=json.Marshal(in.Config)
		if len(configRaw)>16*1024 {
			common.APIError(w,400,"VALIDATION","adapter config exceeds 16 KiB");return
		}
		actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		_,err=a.db.Exec(`INSERT INTO connector.website_adapters(
				partner_id,environment,adapter_type,site_base_url,allowed_domains,capabilities,privacy_mode,enabled,config,updated_by
			) VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb,'AGGREGATED_ONLY',$7,$8::jsonb,$9)
			ON CONFLICT(partner_id,environment) DO UPDATE SET
				adapter_type=EXCLUDED.adapter_type,site_base_url=EXCLUDED.site_base_url,
				allowed_domains=EXCLUDED.allowed_domains,capabilities=EXCLUDED.capabilities,
				privacy_mode='AGGREGATED_ONLY',enabled=EXCLUDED.enabled,config=EXCLUDED.config,
				updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			partnerID,environment,adapterType,baseURL,string(domainsRaw),string(capsRaw),enabled,string(configRaw),actor)
		if err!=nil { common.APIError(w,500,"DB","Could not save website adapter");return }
		a.writeWebsiteAdapter(w,partnerID,environment)
	default:
		common.APIError(w,405,"METHOD","Use GET or PUT")
	}
}

func (a *app) websiteAdapterMap(partnerID,environment string) (map[string]any,error) {
	var adapterType,baseURL,privacyMode,updatedBy string
	var domains,caps,config []byte
	var enabled bool
	var updated time.Time
	err:=a.db.QueryRow(`SELECT adapter_type,site_base_url,allowed_domains,capabilities,privacy_mode,enabled,config,updated_by,updated_at
		FROM connector.website_adapters WHERE partner_id=$1 AND environment=$2`,partnerID,environment).
		Scan(&adapterType,&baseURL,&domains,&caps,&privacyMode,&enabled,&config,&updatedBy,&updated)
	if err!=nil { return nil,err }
	var domainValues []string
	var capabilityValues []string
	var configValue map[string]any
	_ = json.Unmarshal(domains,&domainValues)
	_ = json.Unmarshal(caps,&capabilityValues)
	_ = json.Unmarshal(config,&configValue)
	if configValue==nil { configValue=map[string]any{} }
	return map[string]any{
		"partner_id":partnerID,"environment":environment,"adapter_type":adapterType,
		"site_base_url":baseURL,"allowed_domains":domainValues,
		"capabilities":capabilityValues,"privacy_mode":privacyMode,
		"enabled":enabled,"config":configValue,
		"updated_by":updatedBy,"updated_at":updated,
	},nil
}

func (a *app) writeWebsiteAdapter(w http.ResponseWriter,partnerID,environment string) {
	out,err:=a.websiteAdapterMap(partnerID,environment)
	if err==sql.ErrNoRows {
		common.JSON(w,200,map[string]any{
			"partner_id":partnerID,"environment":environment,"configured":false,
			"adapter_type":"GENERIC_HTTP","site_base_url":"","allowed_domains":[]string{},
			"capabilities":[]string{"ENTITLEMENTS","HEARTBEAT","METRICS","AGGREGATED_DATA","RECONCILIATION"},
			"privacy_mode":"AGGREGATED_ONLY","enabled":false,"config":map[string]any{},
		})
		return
	}
	if err!=nil { common.APIError(w,500,"DB","Could not load website adapter");return }
	out["configured"]=true
	common.JSON(w,200,out)
}

func (a *app) connectorInternalGET(ctx context.Context,host,path string,dst any) error {
	if strings.TrimSpace(host)=="" { return fmt.Errorf("service host is not configured") }
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+host+path,nil)
	if err!=nil{return err}
	common.BindInternalRequest(req,a.internalToken)
	resp,err:=a.client.Do(req)
	if err!=nil{return err}
	defer resp.Body.Close()
	raw,err:=io.ReadAll(io.LimitReader(resp.Body,1<<20))
	if err!=nil{return err}
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("upstream status %d: %s",resp.StatusCode,string(raw))}
	if dst!=nil{return json.Unmarshal(raw,dst)}
	return nil
}

func (a *app) commercialState(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodGet { common.APIError(w,405,"METHOD","Use GET");return }
	cred,err:=a.authenticate(r)
	if err!=nil { common.APIError(w,401,"CONNECTOR_UNAUTHORIZED",err.Error());return }

	adapter,err:=a.websiteAdapterMap(cred.PartnerID,cred.Environment)
	if err==sql.ErrNoRows {
		common.APIError(w,409,"WEBSITE_ADAPTER_REQUIRED","Configure the partner website adapter before reading commercial state")
		return
	}
	if err!=nil { common.APIError(w,500,"DB","Could not load website adapter");return }
	if adapter["enabled"]!=true {
		common.APIError(w,403,"WEBSITE_ADAPTER_DISABLED","Partner website adapter is disabled")
		return
	}

	ctx,cancel:=context.WithTimeout(r.Context(),4*time.Second)
	defer cancel()
	var catalog struct{Items []map[string]any `json:"items"`}
	if err:=a.connectorInternalGET(ctx,a.catalogHost,
		"/internal/v1/partner-portal/"+url.PathEscape(cred.PartnerID)+"/modules",&catalog);err!=nil {
		common.APIError(w,502,"CATALOG_UNAVAILABLE","Could not resolve authoritative module entitlements");return
	}
	var billing map[string]any
	if err:=a.connectorInternalGET(ctx,a.billingHost,
		"/api/v1/billing/partners/"+url.PathEscape(cred.PartnerID)+"/summary",&billing);err!=nil {
		common.APIError(w,502,"BILLING_UNAVAILABLE","Could not resolve authoritative service cycle");return
	}
	var partner map[string]any
	if err:=a.connectorInternalGET(ctx,a.partnersHost,
		"/api/v1/partners/"+url.PathEscape(cred.PartnerID),&partner);err!=nil {
		common.APIError(w,502,"PARTNER_UNAVAILABLE","Could not resolve partner domain binding");return
	}
	var design map[string]any
	if err:=a.connectorInternalGET(ctx,a.cmsHost,
		"/public/v1/cms/partner-design/"+url.PathEscape(cred.PartnerID),&design);err!=nil {
		common.APIError(w,502,"DESIGN_UNAVAILABLE","Could not resolve authoritative partner visual theme");return
	}

	primaryDomain:=normalizeDomain(fmt.Sprint(partner["primary_domain"]))
	if primaryDomain!="" {
		allowed:=false
		for _,domain:=range adapterDomains(adapter["allowed_domains"]){if domain==primaryDomain{allowed=true;break}}
		if !allowed {
			common.APIError(w,409,"DOMAIN_BINDING_MISMATCH","Website adapter does not include the partner primary domain")
			return
		}
	}

	entitlements:=map[string]any{}
	activeCount:=0
	for _,item:=range catalog.Items {
		if fmt.Sprint(item["status"])!="ACTIVE" { continue }
		key:=strings.TrimSpace(fmt.Sprint(item["key"]))
		if key=="" { continue }
		entitlements[key]=map[string]any{
			"status":"ACTIVE",
			"version":item["latest_version"],
			"included_in_base":item["included_in_base"],
		}
		activeCount++
	}

	desired:=map[string]any{"maintenance":map[string]any{},"config":map[string]any{}}
	var maintenanceRaw,configRaw []byte
	if err:=a.db.QueryRowContext(ctx,`SELECT maintenance,config FROM connector.desired_state
		WHERE partner_id=$1 AND environment=$2`,cred.PartnerID,cred.Environment).Scan(&maintenanceRaw,&configRaw);err==nil {
		var maintenance,config map[string]any
		_ = json.Unmarshal(maintenanceRaw,&maintenance)
		_ = json.Unmarshal(configRaw,&config)
		desired["maintenance"]=maintenance
		desired["config"]=config
	}

	publicAdapter:=map[string]any{
		"environment":adapter["environment"],
		"adapter_type":adapter["adapter_type"],
		"site_base_url":adapter["site_base_url"],
		"allowed_domains":adapter["allowed_domains"],
		"capabilities":adapter["capabilities"],
		"privacy_mode":adapter["privacy_mode"],
		"enabled":adapter["enabled"],
		"config":adapter["config"],
	}
	common.JSON(w,200,map[string]any{
		"partner_id":cred.PartnerID,"environment":cred.Environment,
		"contract_version":"START-22.3",
		"tenant_scope":"CREDENTIAL_BOUND",
		"domain_binding":map[string]any{
			"primary_domain":partner["primary_domain"],
			"staging_domain":partner["staging_domain"],
			"website":partner["website"],
		},
		"adapter":publicAdapter,
		"design":design,
		"design_contract_version":"START-23.8",
		"entitlements":entitlements,
		"active_module_count":activeCount,
		"service_cycle":map[string]any{
			"cycle_days":billing["cycle_days"],
			"current_period_start":billing["current_period_start"],
			"current_period_end_exclusive":billing["current_period_end_exclusive"],
			"next_billing_date":billing["next_billing_date"],
		},
		"maintenance":desired["maintenance"],
		"config":desired["config"],
		"data_contract":map[string]any{
			"privacy_mode":"AGGREGATED_ONLY",
			"raw_invoice_data_allowed":false,
			"raw_payment_data_allowed":false,
			"personal_data_default":"MINIMIZED",
			"retention_policy":"7_YEARS",
			"ingest_endpoint":"/connector/v1/data/batches",
			"metrics_endpoint":"/connector/v1/metrics",
			"reconciliation_endpoint":"/connector/v1/reconcile",
		},
		"generated_at":time.Now().UTC(),
	})
}

func adapterDomains(value any) []string {
	out:=[]string{}
	switch raw:=value.(type) {
	case []string:
		for _,x:=range raw{if s:=normalizeDomain(x);s!=""{out=append(out,s)}}
	case []any:
		for _,x:=range raw{if s:=normalizeDomain(fmt.Sprint(x));s!=""{out=append(out,s)}}
	}
	return out
}
