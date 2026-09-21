# START-21 Acceptance — Encrypted backups and verified recovery

## Restore-point scope
- [x] Backup orchestration is isolated in a dedicated private Backups microservice.
- [x] Every restore point captures the isolated partner PostgreSQL database.
- [x] Every restore point captures the partner media/storage namespace.
- [x] Every restore point captures partner/environment/connector configuration without secret-bearing fields.
- [x] Component SHA-256 checksums and byte sizes are persisted in the restore-point manifest.
- [x] Restore-point jobs are persisted in PostgreSQL and survive worker restarts.

## Encryption and offsite storage
- [x] Restore artifacts use chunked AES-256-GCM authenticated encryption.
- [x] The encryption key is injected as a runtime secret and is never committed.
- [x] CI/dev uses a deterministic isolated local offsite adapter.
- [x] Production uses an S3-compatible HTTPS provider through AWS SigV4.
- [x] Production does not treat the HIMATE application/storage disk as offsite backup.
- [x] Offsite ciphertext SHA-256 and byte size are persisted.
- [x] Object-key traversal is rejected.
- [x] Media archive export rejects symlinks/unsafe paths.

## Retention and scheduling
- [x] Backup policy is partner-scoped.
- [x] Retention days are persisted.
- [x] Maximum restore-point count is persisted.
- [x] Automatic backup interval is persisted.
- [x] The scheduler queues due backups from durable policy state.
- [x] Retention pruning deletes the offsite object before marking the restore point EXPIRED.
- [x] Multiple workers claim queue entries with `FOR UPDATE SKIP LOCKED`.

## Mandatory restore verification
- [x] Every successful restore point automatically queues a restore test.
- [x] Restore testing downloads the artifact from the offsite provider rather than using a local staging file.
- [x] Ciphertext checksum is revalidated.
- [x] AES-GCM authentication is revalidated during decryption.
- [x] Manifest and component checksums are revalidated.
- [x] PostgreSQL is restored into a temporary scratch database with `pg_restore`.
- [x] Restored partner identity is verified from `partner_core.system_meta`.
- [x] Media archive is safely extracted and validated.
- [x] Configuration JSON is parsed and partner identity is verified.
- [x] Scratch databases are removed after the test.
- [x] Recoverability is `VERIFIED` only when the latest restore point is READY and its latest restore test PASSED.

## Governance and operations
- [x] Gateway RBAC exposes `backups.read`, `backups.write` and `backups.approve`.
- [x] Backup mutations are approval-grade operations.
- [x] Central audit records restore-point queueing, policy changes, explicit restore tests and retention pruning.
- [x] System Health monitors the Backups service independently.
- [x] System & Operations includes a Backups & Recoverability panel.
- [x] The UI supports restore-point creation, policy editing, explicit restore tests and live status refresh.
- [x] Render Blueprint declares the private Backups service and S3/runtime secret inputs.
- [x] Docker Compose keeps CI/dev offsite data on a separate backup volume.

## Evidence
- `services/cmd/backups/`
- `services/internal/partnerdb/`
- `services/cmd/storage/main.go`
- `services/cmd/gateway/main.go`
- `services/cmd/health/main.go`
- `services/docker/backups.Dockerfile`
- `frontend/lib/backups_panel.dart`
- `docker-compose.yml`
- `render.yaml`
- `scripts/smoke_start_21.sh`
- `docs/adr/0004-encrypted-offsite-backup-and-restore-verification.md`

START-21 is complete only when Go vet/unit/race/build, Flutter analyze/test/release build, the full START-01–20 regression and the START-21 restore smoke are all green.
