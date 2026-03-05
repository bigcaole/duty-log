package handlers

import "testing"

func TestCanAccessOwnedRecord(t *testing.T) {
	if !canAccessOwnedRecord(true, 1, 2) {
		t.Fatalf("admin should access any record")
	}
	if !canAccessOwnedRecord(false, 3, 3) {
		t.Fatalf("owner should access own record")
	}
	if canAccessOwnedRecord(false, 3, 4) {
		t.Fatalf("non-admin should not access others record")
	}
}
