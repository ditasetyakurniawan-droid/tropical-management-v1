package main

import "testing"

func TestCleanChatBody(t *testing.T) {
	body, err := cleanChatBody("  halo tim restoran  ")
	if err != nil || body != "halo tim restoran" {
		t.Fatalf("unexpected clean result: body=%q err=%v", body, err)
	}
	if _, err := cleanChatBody("   "); err == nil {
		t.Fatal("expected empty chat message to fail")
	}
}
