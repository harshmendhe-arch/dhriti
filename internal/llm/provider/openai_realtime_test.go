package provider

import "testing"

func TestBuildRealtimeURLSetsModel(t *testing.T) {
	got, err := buildRealtimeURL("wss://example.com/realtime", "gpt-realtime")
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://example.com/realtime?model=gpt-realtime"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildRealtimeURLPreservesExistingQuery(t *testing.T) {
	got, err := buildRealtimeURL("wss://example.com/realtime?token=abc", "gpt-realtime")
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://example.com/realtime?model=gpt-realtime&token=abc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildRealtimeURLRejectsHTTP(t *testing.T) {
	if _, err := buildRealtimeURL("https://example.com/realtime", "m"); err == nil {
		t.Fatal("expected error for https scheme")
	}
}

func TestGatewayOptionsEnabledTrimsSpace(t *testing.T) {
	if (gatewayOptions{url: "   "}).enabled() {
		t.Fatal("whitespace-only URL should not enable gateway")
	}
	if !(gatewayOptions{url: "wss://x"}).enabled() {
		t.Fatal("non-empty URL should enable gateway")
	}
}
