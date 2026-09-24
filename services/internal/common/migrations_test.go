package common

import "testing"

func TestMigrationChecksumDetectsSourceDrift(t *testing.T) {
	base:=Migration{Version:42,Name:"phase2",Statements:[]string{
		"CREATE TABLE IF NOT EXISTS audit_test(id BIGINT PRIMARY KEY)",
		"ALTER TABLE audit_test ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT ''",
	}}
	same:=Migration{Version:42,Name:"phase2",Statements:append([]string(nil),base.Statements...)}
	changed:=Migration{Version:42,Name:"phase2",Statements:[]string{
		"CREATE TABLE IF NOT EXISTS audit_test(id BIGINT PRIMARY KEY)",
		"ALTER TABLE audit_test ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT 'changed'",
	}}
	if migrationChecksum(base)!=migrationChecksum(same){t.Fatal("equivalent migrations must have identical checksums")}
	if migrationChecksum(base)==migrationChecksum(changed){t.Fatal("SQL drift must change the migration checksum")}
}

func TestMigrationSafetyRejectsDestructiveSchemaChanges(t *testing.T) {
	cases:=[]string{
		"DROP TABLE important_business_data",
		"ALTER TABLE invoices DROP COLUMN total",
		"TRUNCATE invoices",
		"DELETE FROM invoices WHERE status='OLD'",
		"ALTER TABLE invoices RENAME COLUMN total TO amount",
		"ALTER TABLE invoices ALTER COLUMN total TYPE BIGINT",
	}
	for _,stmt:=range cases{
		if err:=validateMigrationSafety(Migration{Version:1,Name:"unsafe",Statements:[]string{stmt}});err==nil{
			t.Fatalf("expected destructive schema statement to be rejected: %s",stmt)
		}
	}
}

func TestMigrationSafetyAllowsExpandOnlyChanges(t *testing.T) {
	m:=Migration{Version:1,Name:"expand",Statements:[]string{
		"CREATE TABLE IF NOT EXISTS invoices(id TEXT PRIMARY KEY)",
		"ALTER TABLE invoices ADD COLUMN IF NOT EXISTS reference TEXT NOT NULL DEFAULT ''",
		"CREATE INDEX IF NOT EXISTS invoices_reference_idx ON invoices(reference)",
	}}
	if err:=validateMigrationSafety(m);err!=nil{t.Fatalf("expand-only migration rejected: %v",err)}
}
