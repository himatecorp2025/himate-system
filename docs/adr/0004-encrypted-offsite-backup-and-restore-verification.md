# ADR-0004: Encrypted durable restore points with mandatory restore verification

- **Status:** Accepted — amended 2026-09-22
- **Date:** 2026-09-21
- **Amended:** 2026-09-22
- **Scope:** START-21 backups and recoverability

## Context

A stored copy is not sufficient evidence of recoverability. HIMATE partner systems contain three recovery-critical classes of state:

1. the physically isolated partner PostgreSQL database,
2. the partner media/storage namespace,
3. partner/environment/connector configuration required to reconstruct runtime state.

A backup must be encrypted, durable across deploys/restarts, isolated from the primary application/storage disk, and test-restorable without overwriting the live partner database.

The original START-21 production profile selected an external S3-compatible provider. The approved 2026-09-22 operational constraint is that the current production stack remains entirely on Render. The production storage profile is therefore amended to use a dedicated Render persistent disk attached only to the Backups service. This intentionally accepts a shared cloud-provider failure domain at the current stage.

## Decision

A dedicated private `backups` microservice owns backup orchestration, retention and restore verification.

Restore-point work is persisted in the control-plane PostgreSQL database. Workers claim queue rows with `FOR UPDATE SKIP LOCKED`, so jobs survive process restarts and can be processed by more than one worker without duplicate claims.

For each restore point the service:

1. runs PostgreSQL `pg_dump --format=custom` against the isolated partner database;
2. streams the partner media namespace from the Storage service;
3. captures partner, environment and connector desired-state configuration while recursively removing secret-bearing keys;
4. records SHA-256 hashes and byte sizes in a manifest;
5. packages the components;
6. encrypts the package with chunked AES-256-GCM;
7. stores the encrypted artifact through the backup-storage provider interface.

Provider implementations are:

- `local` — deterministic CI/development filesystem adapter backed by a separate Docker volume;
- `render_disk` — current production adapter backed by a dedicated Render persistent disk mounted at `/offsite`;
- `s3` — retained optional S3-compatible HTTPS adapter using AWS Signature Version 4 for a future provider-independent replica.

The current Render Blueprint sets `HIMATE_BACKUP_PROVIDER=render_disk`, mounts a dedicated persistent disk at `/offsite`, and does not require S3 endpoint, bucket, access-key or secret-key configuration.

The AES-256 encryption key remains runtime-secret configuration and is never stored in source control or in the restore-point manifest.

## Mandatory restore verification

A successful backup automatically queues a restore test.

The restore worker re-opens the artifact from the configured durable backup provider, validates the ciphertext checksum, authenticates/decrypts every AES-GCM chunk, validates component checksums, and then:

- creates a temporary PostgreSQL scratch database;
- runs `pg_restore --exit-on-error`;
- verifies `partner_core.system_meta.partner_id`;
- safely extracts the media archive with traversal/symlink protections;
- parses the configuration snapshot and verifies its partner identity;
- drops the scratch database.

A restore point is reported as recoverable only when the backup is `READY` and the corresponding restore test is `PASSED`.

## Retention

Each partner has a policy containing:

- retention days,
- maximum restore-point count,
- automatic backup interval,
- enabled/disabled state.

Pruning deletes the durable backup artifact first and only then marks the restore point `EXPIRED`. Failed storage deletion therefore does not create false metadata claiming the stored copy is gone.

## Render production boundary

The production backup disk is distinct from the `himate-storage` disk and is mounted only into the private Backups service. Render persistent storage preserves files across deploys and restarts. Platform disk snapshots provide an additional Render-managed recovery layer.

This is service/disk isolation, not provider-independent offsite disaster recovery. A Render-wide or account-wide failure can affect both primary and backup infrastructure. That trade-off is explicitly accepted for the current Render-only production phase.

## Consequences

- recoverability is continuously proven rather than assumed;
- backup work is isolated from Gateway request latency;
- production no longer depends on AWS/S3 configuration or credentials;
- CI can exercise the entire backup/restore flow without writing to production storage;
- restore testing consumes temporary database/storage capacity and must be capacity-planned;
- encryption-key loss makes existing restore points unrecoverable, so the key itself requires independent secret-management/escrow controls;
- the optional S3 adapter remains available for a later second-provider/offsite replica without changing the restore-point model;
- the current Render-only topology accepts a shared provider failure domain.
