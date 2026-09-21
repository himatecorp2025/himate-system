# ADR-0004: Encrypted offsite restore points with mandatory restore verification

- **Status:** Accepted
- **Date:** 2026-09-21
- **Scope:** START-21 backups and recoverability

## Context

A stored copy is not sufficient evidence of recoverability. HIMATE partner systems contain three recovery-critical classes of state:

1. the physically isolated partner PostgreSQL database,
2. the partner media/storage namespace,
3. partner/environment/connector configuration required to reconstruct runtime state.

The production application/storage disk is not an offsite backup boundary. A backup must also be test-restorable without overwriting the live partner database.

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
7. stores the encrypted artifact through an offsite-provider interface.

Provider implementations are:
- `local` — deterministic CI/development adapter backed by a separate Docker volume;
- `s3` — production S3-compatible HTTPS adapter using AWS Signature Version 4.

The AES-256 key and S3 credentials are runtime secrets. They are never stored in source control or in the restore-point manifest.

## Mandatory restore verification

A successful backup automatically queues a restore test.

The restore worker re-opens the artifact from the configured offsite provider, validates the ciphertext checksum, authenticates/decrypts every AES-GCM chunk, validates component checksums, and then:

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

Pruning deletes the remote object first and only then marks the restore point `EXPIRED`. Failed remote deletion therefore does not create false metadata claiming the offsite copy is gone.

## Consequences

- recoverability is continuously proven rather than assumed;
- backup work is isolated from Gateway request latency;
- production offsite storage is provider-neutral at the service boundary;
- CI can exercise the entire backup/restore flow without writing to production storage;
- restore testing consumes temporary database/storage capacity and must be capacity-planned;
- encryption-key loss makes existing restore points unrecoverable, so the key itself requires independent secret-management/escrow controls;
- S3-compatible credentials and bucket lifecycle/replication remain operational infrastructure responsibilities.
