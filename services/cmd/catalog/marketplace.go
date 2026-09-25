package main

import (
	"context"
	"fmt"
	"himate.local/services/internal/common"
	"strings"
)

type marketplaceSummary struct {
	EN string
	HU string
}

// START-23.11.3 deliberately keeps these summaries high level.
// They describe the catalog purpose of each canonical module without claiming
// implementation details that have not yet been reconstructed from Klavierhaus.
var marketplaceSummaries = map[string]marketplaceSummary{
	"finance":             {"Financial overview for balance-sheet information and financial position reporting.", "Pénzügyi áttekintés a mérlegadatok és a pénzügyi helyzet bemutatásához."},
	"income_statement":    {"Financial reporting workspace for income-statement information.", "Pénzügyi riportfelület az eredménykimutatás adataihoz."},
	"invoice_documents":   {"Workspace for invoice-related documents and invoice records.", "Munkafelület a számlákhoz kapcsolódó dokumentumok és számlaadatok kezeléséhez."},
	"audit_log":           {"Audit-history workspace for reviewing recorded system and business changes.", "Auditnapló a rögzített rendszer- és üzleti változások áttekintéséhez."},
	"backups":             {"Backup-management workspace for backup status and recovery-related records.", "Biztonsági mentési felület a mentések és helyreállítási adatok áttekintéséhez."},
	"pianos":              {"Client-instrument workspace for piano records linked to customers.", "Ügyfélhangszer-felület az ügyfelekhez kapcsolódó zongoraadatok kezeléséhez."},
	"contacts":            {"Client workspace for customer and contact records.", "Ügyfélkezelő felület az ügyfél- és kapcsolattartói adatokhoz."},
	"closed_jobs":         {"Archive of completed service and workshop jobs.", "A lezárt szerviz- és műhelymunkák archívuma."},
	"knowledge_base":      {"Company document archive and internal reference library.", "Vállalati dokumentumarchívum és belső tudástár."},
	"company_data":        {"Company master-data workspace for organizational records.", "Vállalati törzsadat-felület a szervezeti adatok kezeléséhez."},
	"inventory":           {"Inventory workspace for stock, items and inventory records.", "Készletkezelő felület készlet-, tétel- és leltáradatokhoz."},
	"partners":            {"Business-partner workspace for partner and supplier records.", "Üzleti partnerkezelő felület partner- és beszállítói adatokhoz."},
	"planned_jobs":        {"Planning workspace for upcoming service and workshop jobs.", "Tervezési felület a közelgő szerviz- és műhelymunkákhoz."},
	"scheduler":           {"Scheduling workspace for appointments and work planning.", "Ütemezési felület időpontokhoz és munkatervezéshez."},
	"website_services":    {"Service-catalog workspace for company service offerings.", "Szolgáltatáskatalógus a vállalat által kínált szolgáltatásokhoz."},
	"settings":            {"Configuration workspace for organization-level settings.", "Beállítási felület a szervezeti szintű konfigurációhoz."},
	"system_integrations": {"Control workspace for system activation and integrations.", "Vezérlőfelület rendszeraktiváláshoz és integrációkhoz."},
	"users":               {"User-management workspace for organization members.", "Felhasználókezelő felület a szervezet munkatársaihoz."},
	"workshop_workflow":   {"Operational workflow workspace for workshop and service work.", "Operatív workflow felület műhely- és szervizmunkákhoz."},
	"marketing_overview":  {"Overview of marketing campaign activity and performance.", "Áttekintés a marketingkampányok aktivitásáról és teljesítményéről."},
	"customer_inbox":      {"Central workspace for customer messages and inbound communication.", "Központi felület az ügyfélüzenetek és bejövő kommunikáció kezeléséhez."},
	"website_reviews":     {"Review-management workspace for customer review records.", "Értékeléskezelő felület az ügyfélvéleményekhez."},
	"campaigns_utm":       {"Campaign-tracking workspace for campaign and UTM information.", "Kampánykövető felület kampány- és UTM-adatokhoz."},
	"leads":               {"Lead-management workspace for prospects and commercial opportunities.", "Leadkezelő felület érdeklődők és üzleti lehetőségek kezeléséhez."},
	"tracking_cookies":    {"Workspace for tracking and cookie-related configuration and records.", "Felület követési és cookie-beállítások, valamint kapcsolódó adatok kezeléséhez."},
	"seo_keywords":        {"SEO and keyword-management workspace.", "SEO- és kulcsszókezelő felület."},
	"heatmap":             {"Consent-related heatmap and overview workspace.", "Hozzájárulásokhoz kapcsolódó hőtérkép- és áttekintő felület."},
	"website_artists":     {"Artist-content workspace used by website and event operations.", "Előadói tartalomkezelő felület weboldali és eseményfolyamatokhoz."},
	"website_contacts":    {"Website contact and inquiry workspace.", "Weboldali kapcsolat- és érdeklődéskezelő felület."},
	"digital_attendance":  {"Digital attendance and event check-in workspace.", "Digitális jelenléti és eseménybeléptetési felület."},
	"events":              {"Event-management workspace for event records and schedules.", "Eseménykezelő felület eseményadatokhoz és időzítésekhez."},
	"event_guest_list":    {"Guest-list and attendee-data workspace.", "Vendéglista- és résztvevőadat-kezelő felület."},
	"event_invitations":   {"Event invitation management workspace.", "Eseménymeghívó-kezelő felület."},
	"media_library":       {"Central media library for website and event assets.", "Központi médiatár weboldali és eseményeszközökhöz."},
	"pages_content":       {"Website page and content-management workspace.", "Weboldal- és tartalomkezelő felület."},
	"publish_preview":     {"Website preview and publishing workspace.", "Weboldal-előnézeti és publikálási felület."},
	"showroom_pianos":     {"Website and showroom catalog workspace for piano listings.", "Weboldali és bemutatótermi katalógusfelület zongoralistákhoz."},
	"event_tickets":       {"Event ticket and reservation-management workspace.", "Eseményjegy- és foglaláskezelő felület."},
	"needs_assessment":    {"Structured needs-assessment workspace for partner onboarding, discovery and service scoping.", "Strukturált igényfelmérő felület partner-onboardinghoz, felméréshez és szolgáltatási scope meghatározásához."},
	"two_factor_authentication": {"Optional security module for configurable two-factor authentication in Partner Portal environments.", "Opcionális biztonsági modul konfigurálható kétfaktoros azonosításhoz Partnerportál-környezetekben."},
}

func start23113MarketplaceMigration() common.Migration {
	return common.Migration{
		Version: 9,
		Name:    "start-23-11-3-module-marketplace",
		Statements: []string{
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS marketplace_visible BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS marketplace_summary_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS marketplace_summary_hu TEXT NOT NULL DEFAULT ''`,
			`UPDATE catalog.modules SET marketplace_visible=TRUE
				WHERE system=TRUE AND legacy_reference='KLAVIERHAUS_LEGACY'`,
			`CREATE INDEX IF NOT EXISTS catalog_modules_marketplace_idx
				ON catalog.modules(marketplace_visible,publication_status,group_key,module_key)`,
		},
	}
}

func (a *app) seedMarketplaceCatalog(ctx context.Context) error {
	if len(marketplaceSummaries) != len(seedModules) {
		return fmt.Errorf("marketplace summary count %d does not match canonical module count %d", len(marketplaceSummaries), len(seedModules))
	}
	for _, module := range seedModules {
		summary, ok := marketplaceSummaries[module.Key]
		if !ok || strings.TrimSpace(summary.EN) == "" || strings.TrimSpace(summary.HU) == "" {
			return fmt.Errorf("missing marketplace summary for canonical module %s", module.Key)
		}
		if _, err := a.db.ExecContext(ctx, `UPDATE catalog.modules
			SET marketplace_visible=TRUE,marketplace_summary_en=$2,marketplace_summary_hu=$3
			WHERE module_key=$1`, module.Key, summary.EN, summary.HU); err != nil {
			return err
		}
	}
	return nil
}

func marketplaceExecutable(publicationStatus, implementationState, availability string) bool {
	return strings.EqualFold(strings.TrimSpace(publicationStatus), "PUBLISHED") &&
		strings.EqualFold(strings.TrimSpace(implementationState), "READY") &&
		strings.EqualFold(strings.TrimSpace(availability), "ACTIVE")
}

func marketplaceAccessState(status, entitlementState, publicationStatus, implementationState, availability string) string {
	if !strings.EqualFold(strings.TrimSpace(availability), "ACTIVE") {
		return "UNAVAILABLE"
	}
	if !strings.EqualFold(strings.TrimSpace(publicationStatus), "PUBLISHED") ||
		!strings.EqualFold(strings.TrimSpace(implementationState), "READY") {
		return "COMING_SOON"
	}
	if strings.EqualFold(strings.TrimSpace(status), "ACTIVE") &&
		strings.EqualFold(strings.TrimSpace(entitlementState), "ACTIVE") {
		return "ACTIVE"
	}
	return "LOCKED"
}
