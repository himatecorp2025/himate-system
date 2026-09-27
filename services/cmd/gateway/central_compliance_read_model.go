package main

import (
	"context"
	"fmt"
	"net/url"
	"sync"
)

func (a *app) materializeCentralCompliance(ctx context.Context) map[string]any {
	const pageSize = 200
	items := []map[string]any{}
	offset := 0
	for {
		var page central10ItemsPage
		path := fmt.Sprintf("/internal/v1/archives?limit=%d&offset=%d", pageSize, offset)
		if err := a.internalGET(ctx, a.hosts["partners"], path, &page); err != nil {
			return map[string]any{
				"status": "partial", "unavailable": []string{"compliance_archives"},
				"items": []map[string]any{}, "details": map[string]any{},
			}
		}
		items = append(items, page.Items...)
		if !page.HasMore && (page.Total == 0 || len(items) >= page.Total) {
			break
		}
		if len(page.Items) == 0 {
			break
		}
		offset += len(page.Items)
	}

	details := map[string]any{}
	unavailable := []string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, raw := range items {
		partnerID := central10String(raw["partner_id"])
		if partnerID == "" {
			continue
		}
		partnerID := partnerID
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				unavailable = append(unavailable, "archive:"+partnerID)
				mu.Unlock()
				return
			}
			var detail map[string]any
			if err := a.internalGET(
				ctx,
				a.hosts["partners"],
				"/internal/v1/archives/"+url.PathEscape(partnerID),
				&detail,
			); err != nil {
				mu.Lock()
				unavailable = append(unavailable, "archive:"+partnerID)
				mu.Unlock()
				return
			}
			mu.Lock()
			details[partnerID] = detail
			mu.Unlock()
		}()
	}
	wg.Wait()

	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}
	return map[string]any{
		"status": status,
		"unavailable": unavailable,
		"items": items,
		"details": details,
	}
}
