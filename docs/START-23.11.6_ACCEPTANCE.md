# START-23.11.6 Acceptance — Central Notifications

## Purpose

START-23.11.6 establishes one shared HIMATE notification engine.

Modules may emit events, but they do not own independent notification inboxes or read-state implementations.

The delivery pipeline is:

`event → delivery scope → tenant → target user → role permission → effective user-module access → Partner Portal inbox`

## Canonical event categories

The engine recognizes:

- `WORKFLOW`;
- `COMMENT`;
- `DEADLINE`;
- `CALENDAR`;
- `BILLING`;
- `SECURITY`;
- `SYSTEM`.

Event type remains more specific than category, for example `PAYMENT_RETRY_SCHEDULED` and `PARTNER_SUSPENDED_NONPAYMENT` are both Billing notifications.

## Delivery scopes

Every event is explicitly scoped to:

- `PLATFORM`: HIMATE control-plane notification;
- `PARTNER`: Partner Portal notification.

Partner Portal reads never return PLATFORM events.

The HIMATE control-plane feed never returns PARTNER events merely because a platform user has broad permissions.

## Partner isolation and targeting

A PARTNER notification must have a partner ID.

Partner delivery is fail-closed:

1. event partner ID must equal the authenticated Partner Portal tenant;
2. if `target_user_id` is set, it must equal the authenticated user;
3. if `audience_permission` is set, the user's role must contain it;
4. if `module_key` is set, that module must be in the user's **effective** START-23.11.5 module access.

A stored event or read-state row never creates module entitlement or user permission.

## Read state

Read state remains per user in `notifications.read_state`.

Per-item read and read-all re-evaluate exactly the same delivery rules as the feed.

Therefore an authenticated user cannot mark a hidden, cross-tenant, wrong-target, wrong-permission or unassigned-module event as read.

## Producers connected in this phase

### Security

A successful Partner Portal login emits a user-targeted `SECURITY_LOGIN_DETECTED` event.

Notification availability is best-effort and must never block authentication.

### Billing

The existing Billing dunning engine publishes retry, suspension, recovery and operational closure events into the same central engine with:

- `delivery_scope=PARTNER`;
- `category=BILLING`;
- `audience_permission=billing.read`.

The notification event can remain persisted even when a suspended partner cannot currently enter the operational Partner Portal.

## Partner Portal UI

The existing shared Notification Center component is reused.

Partner Portal desktop and responsive headers expose the notification bell when the user has `notifications.read`.

The panel provides:

- unread badge;
- unread-only filter;
- category and severity context;
- per-item mark read;
- mark all read.

## Persistence

- events: `notifications.events`;
- user read state: `notifications.read_state`;
- partner targeting fields are persisted on the central event row.

## Release

`0.8.31-start-23.11.6`

Automated evidence:

- `python3 scripts/audit_start_23_11_6.py`;
- `sh scripts/smoke_start_23_11_6.sh http://127.0.0.1:8080`;
- inherited START-23.11.3–23.11.5 acceptance remains mandatory.
