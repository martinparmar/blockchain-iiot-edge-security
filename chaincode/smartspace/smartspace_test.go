// Unit tests for the SmartSpace chaincode helper logic.
// These tests exercise the pure-Go helpers that do not require a running
// Fabric peer (full integration tests run via the test-network in
// benchmark/). Run with: go test ./...
//
// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestComputeSHA256(t *testing.T) {
	// Known-answer test: SHA-256("") = e3b0c442...
	got := ComputeSHA256([]byte(""))
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("ComputeSHA256(empty) = %s, want %s", got, want)
	}
	if len(got) != 64 {
		t.Errorf("hash length = %d, want 64", len(got))
	}
}

func TestContains(t *testing.T) {
	s := []string{"alice", "bob"}
	if !contains(s, "alice") {
		t.Error("expected contains to find alice")
	}
	if contains(s, "carol") {
		t.Error("did not expect contains to find carol")
	}
}

func TestRemove(t *testing.T) {
	s := []string{"alice", "bob", "carol"}
	out := remove(s, "bob")
	if contains(out, "bob") {
		t.Error("bob should have been removed")
	}
	if len(out) != 2 {
		t.Errorf("length after remove = %d, want 2", len(out))
	}
}
