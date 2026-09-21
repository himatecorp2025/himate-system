# ADR-0002: Server-render published CMS content at the Gateway

- **Status:** Accepted
- **Date:** 2026-09-21
- **Scope:** START-17 public marketing/SEO

## Context

The public marketing pages were source-controlled HTML enhanced by browser JavaScript that fetched published CMS data. Search crawlers could therefore receive fallback title/meta/content in the initial response instead of the published CMS version.

## Decision

The Gateway renders the active PUBLISHED CMS snapshot into the initial HTML response for public marketing routes.

The private CMS service remains the content authority. The Gateway requests only the public published contract and renders title, meta description, canonical, robots, Open Graph data and visible section content. Hidden sections are removed before response delivery.

Browser JavaScript remains only a hydration/fallback layer.

## Failure mode

If CMS is temporarily unavailable, source-controlled static HTML is served with `X-Himate-SSR: static-fallback` rather than taking the public site offline.

## SEO discovery

`/sitemap.xml` is generated from the published CMS manifest and excludes noindex pages. `/robots.txt` advertises the sitemap.

## Consequences

- search engines see published SEO/content immediately
- no extra public frontend microservice is required
- CMS remains private
- Gateway public-page latency now includes a bounded CMS read, so timeout/fallback behavior is mandatory
- future caching can be added at this boundary without changing CMS ownership
