package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"himate.local/services/internal/common"
)

// adminPartnerLogo accepts a partner-scoped logo upload from the HIMATE control
// plane, stores it through CMS partner media, marks the asset as the partner's
// public logo, and persists the resulting public URL in the partner master data.
func (a *app) adminPartnerLogo(w http.ResponseWriter, r *http.Request, actor user) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/partners/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "logo" {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner logo route not found")
		return
	}
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	partnerID := parts[0]

	var partner map[string]any
	if err := a.internalGET(r.Context(), a.hosts["partners"], "/api/v1/partners/"+url.PathEscape(partnerID), &partner); err != nil {
		common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner not found")
		return
	}

	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		common.APIError(w, http.StatusBadGateway, "CMS_UNAVAILABLE", "CMS service is not configured")
		return
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if !strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		common.APIError(w, http.StatusBadRequest, "MULTIPART_REQUIRED", "Partner logo must be uploaded as multipart/form-data")
		return
	}

	upstream := "/internal/v1/cms/partner-media/" + url.PathEscape(partnerID)
	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		"http://"+host+upstream,
		io.LimitReader(r.Body, 66<<20),
	)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "REQUEST", "Could not prepare partner logo upload")
		return
	}
	common.BindInternalRequest(req, a.internalToken)
	req.Header.Set("X-Himate-User-ID", actor.ID)
	req.Header.Set("X-Correlation-ID", strings.TrimSpace(r.Header.Get("X-Correlation-ID")))
	req.Header.Set("Content-Type", contentType)

	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		common.APIError(w, http.StatusBadGateway, "CMS_UNAVAILABLE", "Partner logo upload failed")
		return
	}
	defer resp.Body.Close()
	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		common.APIError(w, http.StatusBadGateway, "CMS_RESPONSE", "Could not read partner logo response")
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if len(rawBody) > 0 {
			_, _ = w.Write(rawBody)
		}
		return
	}

	var media map[string]any
	if err := json.Unmarshal(rawBody, &media); err != nil {
		common.APIError(w, http.StatusBadGateway, "CMS_RESPONSE", "Partner logo response was invalid")
		return
	}
	mediaID := strings.TrimSpace(fmt.Sprint(media["id"]))
	publicURL := strings.TrimSpace(fmt.Sprint(media["public_url"]))
	if mediaID == "" || publicURL == "" {
		common.APIError(w, http.StatusBadGateway, "CMS_RESPONSE", "Partner logo response did not contain a public asset")
		return
	}

	var updated map[string]any
	err = a.internalJSON(
		r.Context(),
		http.MethodPatch,
		a.hosts["partners"],
		"/api/v1/partners/"+url.PathEscape(partnerID),
		map[string]any{"logo_url": publicURL},
		map[string]string{"X-Himate-User-ID": actor.ID},
		&updated,
	)
	if err != nil {
		writeInternalError(w, err, "Partner logo could not be linked to the partner master record")
		return
	}

	common.JSON(w, http.StatusCreated, map[string]any{
		"partner_id": partnerID,
		"logo_url": publicURL,
		"media_id": mediaID,
		"media": media,
		"partner": updated,
	})
}
