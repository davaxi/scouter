package postprocess

import (
	"context"
	"strconv"
	"strings"
	"time"

	"scouter-crawler/internal/db"
)

// SitemapAnalysisCH is the ClickHouse-only counterpart of sitemapAnalysis, run
// when the PG post-processing is skipped (CLICKHOUSE_DROP_PG). It records the
// sitemap ids in `sitemap_urls` (read by the CH buildMetrics for in_sitemap),
// appends sitemap-only placeholder pages at depth=-1 (the CH read shim derives
// in_crawl from depth >= 0), then fetches the new in-scope URLs. Must run before
// CHRunner.Run so page_metrics covers the sitemap pages.
func (r *Runner) SitemapAnalysisCH(ctx context.Context, ch *db.CH) error {
	idToURL, allowed, err := r.loadSitemap(ctx)
	if err != nil || idToURL == nil {
		return err
	}
	return r.applySitemapCH(ctx, ch, idToURL, allowed)
}

func (r *Runner) applySitemapCH(ctx context.Context, ch *db.CH, idToURL map[string]string, allowed []string) error {
	cid := strconv.Itoa(r.crawlID)
	smTable := ch.DB() + ".sitemap_urls"

	if err := ch.DropPartition(ctx, smTable, r.crawlID); err != nil {
		return err
	}
	ids := make([]any, 0, len(idToURL))
	for id := range idToURL {
		ids = append(ids, map[string]any{"crawl_id": r.crawlID, "id": id})
	}
	if err := ch.InsertJSONEachRow(ctx, smTable, ids); err != nil {
		return err
	}

	rows, err := ch.QueryTSV(ctx, "SELECT DISTINCT id FROM "+ch.DB()+".pages WHERE crawl_id = "+cid+
		" AND id IN (SELECT id FROM "+smTable+" WHERE crawl_id = "+cid+")")
	if err != nil {
		return err
	}
	existing := make(map[string]bool, len(rows))
	for _, row := range rows {
		if len(row) > 0 {
			existing[strings.TrimSpace(row[0])] = true
		}
	}

	// Placeholders are dated slightly in the past so that ReplacingMergeTree(date)
	// keeps the row written by the fetch pass below. Unix seconds, so the value
	// doesn't depend on the crawler's vs the server's timezone.
	date := time.Now().Add(-time.Minute).Unix()
	var placeholders []any
	var newInScopeURLs []string
	for id, u := range idToURL {
		if existing[id] {
			continue
		}
		inScope := urlInScope(u, allowed)
		if inScope {
			newInScopeURLs = append(newInScopeURLs, u)
		}
		placeholders = append(placeholders, map[string]any{
			"crawl_id": r.crawlID, "id": id, "date": date, "domain": smDomain(u), "url": truncate(u, 2083),
			"depth": -1, "code": 0, "crawled": 0, "external": b2iCH(!inScope), "blocked": 0,
		})
	}
	if len(placeholders) > 0 {
		if err := ch.InsertJSONEachRow(ctx, ch.DB()+".pages", placeholders); err != nil {
			return err
		}
	}
	r.logf("sitemap: %d already crawled, %d sitemap-only (%d in scope to fetch)",
		len(existing), len(placeholders), len(newInScopeURLs))

	if len(newInScopeURLs) > 0 && r.SitemapFetch != nil {
		if err := r.SitemapFetch(ctx, newInScopeURLs, allowed); err != nil {
			r.logf("sitemap fetch error: %v", err)
		}
	}
	return nil
}
