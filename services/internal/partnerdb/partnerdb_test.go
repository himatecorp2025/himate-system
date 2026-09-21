package partnerdb

import "testing"

func TestDatabaseName(t *testing.T) {
	cases := map[string]string{
		"ptr_000123": "himate_ptr_000123",
		"PARTNER-ABC": "himate_partner_abc",
		"../bad": "",
		"bad space": "",
	}
	for input, want := range cases {
		if got := DatabaseName(input); got != want {
			t.Fatalf("%q: expected %q, got %q", input, want, got)
		}
	}
}

func TestAdminAndDatabaseDSN(t *testing.T) {
	base := "postgres://user:secret@postgres:5432/himate?sslmode=disable"
	admin, err := AdminDSN(base)
	if err != nil {
		t.Fatal(err)
	}
	if admin != "postgres://user:secret@postgres:5432/postgres?sslmode=disable" {
		t.Fatalf("unexpected admin DSN: %s", admin)
	}
	db, err := DatabaseDSN(base, "ptr_000123")
	if err != nil {
		t.Fatal(err)
	}
	if db != "postgres://user:secret@postgres:5432/himate_ptr_000123?sslmode=disable" {
		t.Fatalf("unexpected partner DSN: %s", db)
	}
}
