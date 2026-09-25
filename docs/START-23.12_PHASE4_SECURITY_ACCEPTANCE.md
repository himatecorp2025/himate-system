# START-23.12 Phase 4 — Security Acceptance

## Objective

Phase 4 hardens identity, browser-origin and private service trust boundaries without rewriting commercial, provisioning, payment or tenant business workflows.

The rebuild intentionally favors a small number of stable security invariants over a large zero-trust rule matrix.

## Security model

### Browser and identity boundary

- Existing signed HttpOnly session cookies and session-version invalidation remain the session model.
- Password policy remains at least 12 characters with mixed character classes.
- SameSite=Strict cookies, exact-origin checks and Sec-Fetch-Site checks reject cross-site browser mutations.
- Non-browser API clients that send neither browser-origin header remain compatible with existing CI and operational tooling.
- Client-supplied HIMATE authority headers are stripped at the Gateway and reconstructed from the authenticated admin or Partner Portal session.
- Existing security headers remain active, including CSP, HSTS on HTTPS, X-Frame-Options and nosniff.

### Multi-factor authentication

- TOTP MFA uses a short-lived, five-attempt challenge.
- MFA secrets are encrypted at rest using an AES-GCM key derived from the Gateway session secret.
- Administrator MFA and sensitive Partner Portal roles (owner/admin/billing) are required when HIMATE_MFA_REQUIRED=true.
- Render production sets HIMATE_MFA_REQUIRED=true.
- Docker Compose sets it to false so inherited historical smoke suites continue to validate business behavior without interactive TOTP enrollment.
- The Flutter admin and Partner Portal login flows handle initial enrollment and subsequent verification.

### Private service identity

- HIMATE_INTERNAL_TOKEN remains the private-network root credential.
- Every internal service request is HMAC-signed centrally by common.DoInternal at the final transport boundary.
- The signature covers method, path, query, caller identity and HIMATE authority headers.
- Each service has a fixed caller ID; no per-service signing key inventory is introduced.
- Private /internal/v1 routes require a valid timestamped service signature when HIMATE_REQUIRE_SERVICE_SIGNATURE=true.
- Render production enables this verification for private services.
- Docker Compose keeps HIMATE_REQUIRE_SERVICE_SIGNATURE=false because inherited Phase 2/3/3B acceptance tools intentionally call private endpoints directly with the root internal credential. The signature contract itself is exercised by Go unit tests.
- Feature/business code does not perform request signing itself.

This is deliberately simpler than the discarded Phase 4 implementation. It prevents public header spoofing and unsigned internal control-plane access while avoiding manual signature ordering, per-route caller matrices and cross-service key synchronization.

## Tenant and financial data

Phase 4 does not add blanket database foreign keys or generic ownership triggers to every partner_id table. Those mechanisms caused historical fixtures and valid recovery workflows to fail without materially improving browser/API authorization.

Tenant isolation remains enforced where authority is established: Partner Portal tenant identity comes exclusively from the authenticated partner session, admin permissions remain Gateway-controlled, and downstream services receive trusted headers only from Gateway/private service calls.

Existing Billing, Payments and Evidence integrity checks are preserved unchanged. Phase 4 does not relax payment validation, Evidence file hashing, commercial agreement rules, license collection or provisioning gates.

## Acceptance gates

The Phase 4 architecture audit verifies the security invariants above and rejects reintroduction of feature-level signing or blanket tenant-firewall code.

The Phase 4 runtime smoke verifies security headers, cross-site login rejection, originless API compatibility and inherited admin login behavior. All earlier START runtime smokes continue to run before Phase 4 and therefore remain the regression proof for commercial and tenant workflows.
