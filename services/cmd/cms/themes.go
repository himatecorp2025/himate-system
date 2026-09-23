package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

type visualTheme struct {
	LayoutKey    string            `json:"layout_key"`
	Navy         string            `json:"navy"`
	Gold         string            `json:"gold"`
	Background   string            `json:"background"`
	TextColor    string            `json:"text_color"`
	HeadingFont  string            `json:"heading_font"`
	BodyFont     string            `json:"body_font"`
	ButtonRadius int               `json:"button_radius"`
	Assets       map[string]string `json:"assets"`
}

type designProfileRow struct {
	ID             string
	OwnerType      string
	OwnerID        string
	Name           string
	Description    string
	ThemeRaw       []byte
	CatalogVisible bool
	CreatedBy      string
	UpdatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func defaultVisualTheme(layout string) visualTheme {
	switch strings.TrimSpace(layout) {
	case "modern_grid":
		return visualTheme{
			LayoutKey: "modern_grid", Navy: "#10233F", Gold: "#C99A45",
			Background: "#F4F6F8", TextColor: "#182230",
			HeadingFont: "Inter", BodyFont: "Inter", ButtonRadius: 12,
			Assets: map[string]string{},
		}
	case "minimal":
		return visualTheme{
			LayoutKey: "minimal", Navy: "#111827", Gold: "#B8893C",
			Background: "#FFFFFF", TextColor: "#111827",
			HeadingFont: "Georgia", BodyFont: "Arial", ButtonRadius: 2,
			Assets: map[string]string{},
		}
	default:
		return visualTheme{
			LayoutKey: "classic_editorial", Navy: "#06172C", Gold: "#D7AE62",
			Background: "#F8F9FB", TextColor: "#1F2937",
			HeadingFont: "Cormorant Garamond", BodyFont: "Inter", ButtonRadius: 6,
			Assets: map[string]string{},
		}
	}
}

func normalizeVisualTheme(in visualTheme) visualTheme {
	in.LayoutKey = strings.TrimSpace(in.LayoutKey)
	if in.LayoutKey == "" {
		in.LayoutKey = "classic_editorial"
	}
	in.Navy = strings.ToUpper(strings.TrimSpace(in.Navy))
	in.Gold = strings.ToUpper(strings.TrimSpace(in.Gold))
	in.Background = strings.ToUpper(strings.TrimSpace(in.Background))
	in.TextColor = strings.ToUpper(strings.TrimSpace(in.TextColor))
	in.HeadingFont = strings.TrimSpace(in.HeadingFont)
	in.BodyFont = strings.TrimSpace(in.BodyFont)
	if in.Assets == nil {
		in.Assets = map[string]string{}
	}
	clean := map[string]string{}
	for slot, mediaID := range in.Assets {
		slot = strings.TrimSpace(slot)
		mediaID = strings.TrimSpace(mediaID)
		if slot != "" && mediaID != "" {
			clean[slot] = mediaID
		}
	}
	in.Assets = clean
	return in
}

func (a *app) partnerOwnsMedia(ctx context.Context, partnerID, mediaID string) bool {
	if strings.TrimSpace(mediaID) == "" {
		return true
	}
	var exists bool
	_ = a.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM cms.media_assets WHERE id=$1 AND owner_type='PARTNER' AND owner_id=$2)",
		mediaID, partnerID,
	).Scan(&exists)
	return exists
}

func (a *app) validateVisualTheme(ctx context.Context, partnerID string, in visualTheme) error {
	in = normalizeVisualTheme(in)
	if !designLayouts[in.LayoutKey] {
		return fmt.Errorf("unsupported layout family")
	}
	if !designColorPattern.MatchString(in.Navy) ||
		!designColorPattern.MatchString(in.Gold) ||
		!designColorPattern.MatchString(in.Background) ||
		!designColorPattern.MatchString(in.TextColor) {
		return fmt.Errorf("brand colors must use six-digit hexadecimal values")
	}
	if !designFonts[in.HeadingFont] || !designFonts[in.BodyFont] {
		return fmt.Errorf("unsupported design font")
	}
	if in.ButtonRadius < 0 || in.ButtonRadius > 40 {
		return fmt.Errorf("button radius must be between 0 and 40")
	}
	for slot, mediaID := range in.Assets {
		if !designAssetSlots[slot] {
			return fmt.Errorf("unsupported design asset slot %q", slot)
		}
		if !a.partnerOwnsMedia(ctx, partnerID, mediaID) {
			return fmt.Errorf("design asset %q does not belong to this partner", slot)
		}
	}
	return nil
}

func designProfileSelect() string {
	return `SELECT id,owner_type,owner_id,name,description,theme,catalog_visible,created_by,updated_by,created_at,updated_at
		FROM cms.design_profiles`
}

func scanDesignProfile(s scanner) (designProfileRow, error) {
	var row designProfileRow
	err := s.Scan(&row.ID, &row.OwnerType, &row.OwnerID, &row.Name, &row.Description, &row.ThemeRaw,
		&row.CatalogVisible, &row.CreatedBy, &row.UpdatedBy, &row.CreatedAt, &row.UpdatedAt)
	return row, err
}

func decodeVisualTheme(raw []byte) visualTheme {
	theme := defaultVisualTheme("classic_editorial")
	if len(raw) > 2 {
		_ = json.Unmarshal(raw, &theme)
	}
	return normalizeVisualTheme(theme)
}

func designProfileMap(row designProfileRow) map[string]any {
	return map[string]any{
		"id": row.ID,
		"owner_type": row.OwnerType,
		"owner_id": row.OwnerID,
		"name": row.Name,
		"description": row.Description,
		"theme": decodeVisualTheme(row.ThemeRaw),
		"catalog_visible": row.CatalogVisible,
		"created_by": row.CreatedBy,
		"updated_by": row.UpdatedBy,
		"created_at": row.CreatedAt.UTC(),
		"updated_at": row.UpdatedAt.UTC(),
	}
}

func (a *app) getDesignProfile(id string) (designProfileRow, error) {
	return scanDesignProfile(a.db.QueryRow(designProfileSelect()+" WHERE id=$1", id))
}

func (a *app) listPartnerDesignProfiles(partnerID string) ([]map[string]any, error) {
	rows, err := a.db.Query(designProfileSelect()+
		" WHERE (owner_type='CATALOG' AND catalog_visible=TRUE) OR (owner_type='PARTNER' AND owner_id=$1) "+
		"ORDER BY CASE WHEN owner_type='CATALOG' THEN 0 ELSE 1 END,name,id", partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		row, scanErr := scanDesignProfile(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, designProfileMap(row))
	}
	return out, rows.Err()
}

func (a *app) activePartnerDesignProfile(partnerID string) (designProfileRow, error) {
	var profileID string
	err := a.db.QueryRow("SELECT active_profile_id FROM cms.design_scope_state WHERE scope_type='PARTNER' AND scope_id=$1", partnerID).Scan(&profileID)
	if err != nil {
		profileID = "theme_classic_editorial"
	}
	row, getErr := a.getDesignProfile(profileID)
	if getErr == nil {
		if row.OwnerType == "CATALOG" && row.CatalogVisible {
			return row, nil
		}
		if row.OwnerType == "PARTNER" && row.OwnerID == partnerID {
			return row, nil
		}
	}
	return a.getDesignProfile("theme_classic_editorial")
}

func (a *app) partnerDesignPayload(partnerID string) (map[string]any, error) {
	profiles, err := a.listPartnerDesignProfiles(partnerID)
	if err != nil {
		return nil, err
	}
	active, err := a.activePartnerDesignProfile(partnerID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"partner_id": partnerID,
		"active_profile_id": active.ID,
		"active_profile": designProfileMap(active),
		"effective_theme": decodeVisualTheme(active.ThemeRaw),
		"profiles": profiles,
		"content_binding": "UNCHANGED",
		"mechanics_binding": "UNCHANGED",
	}, nil
}

func validPartnerDesignID(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "ptr_") && len(value) <= 80 && !strings.Contains(value, "/")
}

func (a *app) partnerDesignInternal(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/cms/partner-design/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 0 || !validPartnerDesignID(parts[0]) {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner design scope not found")
		return
	}
	partnerID := parts[0]

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
			return
		}
		out, err := a.partnerDesignPayload(partnerID)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load partner design profiles")
			return
		}
		common.JSON(w, http.StatusOK, out)
		return
	}

	if len(parts) == 2 && parts[1] == "profiles" {
		if r.Method != http.MethodPost {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
			return
		}
		var in struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			Theme       visualTheme `json:"theme"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid request")
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Description = strings.TrimSpace(in.Description)
		in.Theme = normalizeVisualTheme(in.Theme)
		if len(in.Name) < 2 || len(in.Name) > 100 || len(in.Description) > 500 {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", "Profile name must be 2-100 characters and description at most 500 characters")
			return
		}
		if err := a.validateVisualTheme(r.Context(), partnerID, in.Theme); err != nil {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
			return
		}
		id := newID("theme_partner_")
		_, err := a.db.ExecContext(r.Context(), `INSERT INTO cms.design_profiles(
			id,owner_type,owner_id,name,description,theme,catalog_visible,created_by,updated_by
		) VALUES($1,'PARTNER',$2,$3,$4,$5::jsonb,FALSE,$6,$6)`,
			id, partnerID, in.Name, in.Description, string(jsonBytes(in.Theme)), actor(r))
		if err != nil {
			common.APIError(w, http.StatusConflict, "PROFILE_CONFLICT", "Partner design profile could not be created")
			return
		}
		row, _ := a.getDesignProfile(id)
		_ = a.audit(r.Context(), "", id, "PARTNER_THEME_PROFILE_CREATED", actor(r), correlationID(r), map[string]any{}, designProfileMap(row))
		common.JSON(w, http.StatusCreated, designProfileMap(row))
		return
	}

	if len(parts) >= 3 && parts[1] == "profiles" {
		profileID := strings.TrimSpace(parts[2])
		row, err := a.getDesignProfile(profileID)
		if err != nil || row.OwnerType != "PARTNER" || row.OwnerID != partnerID {
			if len(parts) == 4 && parts[3] == "activate" {
				row, err = a.getDesignProfile(profileID)
				if err != nil || row.OwnerType != "CATALOG" || !row.CatalogVisible {
					common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Design profile not found")
					return
				}
			} else {
				common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Design profile not found")
				return
			}
		}

		if len(parts) == 3 {
			if r.Method != http.MethodPut {
				common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use PUT")
				return
			}
			if row.OwnerType != "PARTNER" || row.OwnerID != partnerID {
				common.APIError(w, http.StatusForbidden, "FORBIDDEN", "Catalog themes cannot be edited by partners")
				return
			}
			var in struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Theme       visualTheme `json:"theme"`
			}
			if common.Decode(r, &in) != nil {
				common.APIError(w, http.StatusBadRequest, "JSON", "Invalid request")
				return
			}
			in.Name = strings.TrimSpace(in.Name)
			in.Description = strings.TrimSpace(in.Description)
			in.Theme = normalizeVisualTheme(in.Theme)
			if len(in.Name) < 2 || len(in.Name) > 100 || len(in.Description) > 500 {
				common.APIError(w, http.StatusBadRequest, "VALIDATION", "Invalid profile name or description")
				return
			}
			if err := a.validateVisualTheme(r.Context(), partnerID, in.Theme); err != nil {
				common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
				return
			}
			old := designProfileMap(row)
			_, err = a.db.ExecContext(r.Context(), `UPDATE cms.design_profiles SET
				name=$3,description=$4,theme=$5::jsonb,updated_by=$6,updated_at=NOW()
				WHERE id=$1 AND owner_id=$2 AND owner_type='PARTNER'`,
				profileID, partnerID, in.Name, in.Description, string(jsonBytes(in.Theme)), actor(r))
			if err != nil {
				common.APIError(w, http.StatusConflict, "PROFILE_CONFLICT", "Partner design profile could not be updated")
				return
			}
			next, _ := a.getDesignProfile(profileID)
			_ = a.audit(r.Context(), "", profileID, "PARTNER_THEME_PROFILE_UPDATED", actor(r), correlationID(r), old, designProfileMap(next))
			common.JSON(w, http.StatusOK, designProfileMap(next))
			return
		}

		if len(parts) == 4 && parts[3] == "activate" {
			if r.Method != http.MethodPost {
				common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
				return
			}
			old, _ := a.activePartnerDesignProfile(partnerID)
			_, err := a.db.ExecContext(r.Context(), `INSERT INTO cms.design_scope_state(scope_type,scope_id,active_profile_id,updated_by,updated_at)
				VALUES('PARTNER',$1,$2,$3,NOW())
				ON CONFLICT(scope_type,scope_id) DO UPDATE SET active_profile_id=EXCLUDED.active_profile_id,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
				partnerID, profileID, actor(r))
			if err != nil {
				common.APIError(w, http.StatusInternalServerError, "DB", "Could not activate design profile")
				return
			}
			next, _ := a.activePartnerDesignProfile(partnerID)
			_ = a.audit(r.Context(), "", profileID, "PARTNER_THEME_ACTIVATED", actor(r), correlationID(r),
				map[string]any{"active_profile_id": old.ID},
				map[string]any{"active_profile_id": next.ID, "content_binding": "UNCHANGED", "mechanics_binding": "UNCHANGED"})
			payload, _ := a.partnerDesignPayload(partnerID)
			common.JSON(w, http.StatusOK, payload)
			return
		}
	}

	common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner design action not found")
}

func (a *app) publicPartnerDesign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or HEAD")
		return
	}
	partnerID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/cms/partner-design/"), "/")
	if !validPartnerDesignID(partnerID) {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner design not found")
		return
	}
	active, err := a.activePartnerDesignProfile(partnerID)
	if err != nil {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner design not found")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"partner_id": partnerID,
		"profile_id": active.ID,
		"profile_name": active.Name,
		"theme": decodeVisualTheme(active.ThemeRaw),
	})
}

func (a *app) partnerMediaInternal(w http.ResponseWriter, r *http.Request) {
	partnerID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/cms/partner-media/"), "/")
	if !validPartnerDesignID(partnerID) {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner media scope not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(mediaSelect()+" WHERE owner_type='PARTNER' AND owner_id=$1 ORDER BY created_at DESC LIMIT 500", partnerID)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load partner media")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			if media, scanErr := scanMedia(rows); scanErr == nil {
				items = append(items, mapMedia(media))
			}
		}
		common.JSON(w, http.StatusOK, map[string]any{"partner_id": partnerID, "items": items, "count": len(items)})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxMediaBytes+(1<<20))
		if err := r.ParseMultipartForm(maxMediaBytes + (1 << 20)); err != nil {
			common.APIError(w, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "Partner design media request exceeds 65 MiB")
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			common.APIError(w, http.StatusBadRequest, "FILE_REQUIRED", "file is required")
			return
		}
		defer file.Close()
		tmp, err := os.CreateTemp("", "himate-partner-design-*")
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "FILE", "Could not prepare design media")
			return
		}
		tmpName := tmp.Name()
		defer func() {
			tmp.Close()
			os.Remove(tmpName)
		}()
		hash := sha256.New()
		size, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(file, maxMediaBytes+1))
		if err != nil || size > maxMediaBytes {
			common.APIError(w, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "Partner design media exceeds 64 MiB")
			return
		}
		if _, err = tmp.Seek(0, 0); err != nil {
			common.APIError(w, http.StatusInternalServerError, "FILE", "Could not inspect design media")
			return
		}
		buf := make([]byte, 512)
		readN, _ := tmp.Read(buf)
		mime := http.DetectContentType(buf[:readN])
		if !cmsMimeAllowed(mime) || !strings.HasPrefix(mime, "image/") {
			common.APIError(w, http.StatusUnsupportedMediaType, "MEDIA_TYPE", "Partner design assets must be PNG, JPEG or WebP")
			return
		}
		if _, err = tmp.Seek(0, 0); err != nil {
			common.APIError(w, http.StatusInternalServerError, "FILE", "Could not store design media")
			return
		}
		id := newID("cms_media_")
		ext := ".bin"
		switch mime {
		case "image/png":
			ext = ".png"
		case "image/jpeg":
			ext = ".jpg"
		case "image/webp":
			ext = ".webp"
		}
		key := "partner/" + partnerID + "/" + id + ext
		sum := hex.EncodeToString(hash.Sum(nil))
		stored, err := a.putMedia(r.Context(), key, tmp, size)
		if err != nil {
			common.APIError(w, http.StatusBadGateway, "STORAGE", err.Error())
			return
		}
		if got := strings.TrimSpace(fmt.Sprint(stored["sha256"])); got != "" && got != sum {
			common.APIError(w, http.StatusBadGateway, "CHECKSUM_MISMATCH", "Stored design media checksum mismatch")
			return
		}
		alt := strings.TrimSpace(r.FormValue("alt_text"))
		if len(alt) > 240 {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", "alt_text is too long")
			return
		}
		_, err = a.db.ExecContext(r.Context(), `INSERT INTO cms.media_assets(
			id,original_filename,mime_type,object_namespace,object_key,size_bytes,sha256,alt_text,created_by,owner_type,owner_id
		) VALUES($1,$2,$3,'_cms',$4,$5,$6,$7,$8,'PARTNER',$9)`,
			id, safeFilename(header.Filename), mime, key, size, sum, alt, actor(r), partnerID)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not record partner design media")
			return
		}
		purpose := strings.ToLower(strings.TrimSpace(r.FormValue("purpose")))
		if purpose != "" && purpose != "logo" {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", "Unsupported partner media purpose")
			return
		}
		if purpose == "logo" {
			_, err = a.db.ExecContext(r.Context(), `INSERT INTO cms.partner_brand_assets(partner_id,slot,media_id,updated_by,updated_at)
				VALUES($1,'logo',$2,$3,NOW())
				ON CONFLICT(partner_id,slot) DO UPDATE SET media_id=EXCLUDED.media_id,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
				partnerID, id, actor(r))
			if err != nil {
				common.APIError(w, http.StatusInternalServerError, "DB", "Could not register partner logo")
				return
			}
		}
		media, _ := a.getMedia(id)
		out := mapMedia(media)
		if purpose == "logo" {
			out["purpose"] = "logo"
			out["public_url"] = "/public/v1/cms/media/" + id
		}
		_ = a.audit(r.Context(), "", id, "PARTNER_DESIGN_MEDIA_UPLOADED", actor(r), correlationID(r),
			map[string]any{}, map[string]any{"partner_id": partnerID, "media_id": id, "purpose": purpose, "sha256": sum})
		common.JSON(w, http.StatusCreated, out)
	default:
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or POST")
	}
}

func activeThemeAssetIDs(theme visualTheme) []string {
	set := map[string]bool{}
	for _, id := range theme.Assets {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = true
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
