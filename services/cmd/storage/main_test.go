package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestObjectPathStaysInsideNamespace(t *testing.T) {
	a := &app{root: t.TempDir()}
	path, err := a.objectPath("ptr_1", "evidence/evd_1.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, filepath.Join("ptr_1", "evidence")) {
		t.Fatalf("unexpected object path %q", path)
	}
	if _, err := a.objectPath("ptr_1", "../escape.txt"); err == nil {
		t.Fatal("path traversal must be rejected")
	}
}
