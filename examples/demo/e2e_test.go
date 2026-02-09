package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gabrielmiguelok/golivekit/client"
	"github.com/gabrielmiguelok/golivekit/pkg/router"
	"github.com/gabrielmiguelok/golivekit/pkg/transport"
)

func TestDocsNavigation_E2E(t *testing.T) {
	// Enable WebSocket debug logging
	transport.DebugWebSocket = true

	// Create a test server with the same routes as main
	r := router.New()
	r.Live("/_live/websocket", NewDemo)
	r.Handle("/_live/", http.StripPrefix("/_live/", client.Handler()))
	r.Live("/", NewDemo)
	r.Live("/docs", NewDocs)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// 1. First, verify HTTP response has data-slot elements
	resp, err := http.Get(ts.URL + "/docs")
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	// Read body
	buf := make([]byte, 64*1024)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, `data-slot="content"`) {
		t.Error("Missing data-slot='content' in response")
	}
	if !strings.Contains(body, `data-slot="sidebar"`) {
		t.Error("Missing data-slot='sidebar' in response")
	}
	if !strings.Contains(body, `data-live-view="docs"`) {
		t.Error("Missing data-live-view='docs' in response")
	}

	t.Log("✓ HTTP response contains required data-slot elements")

	// 2. Connect via WebSocket using same library as server
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/docs"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "test done")

	t.Log("✓ WebSocket connected")

	// 3. Send phx_join - topic doesn't matter, server assigns it
	joinMsg := map[string]any{
		"ref":      "1",
		"join_ref": "1",
		"topic":    "lv:docs",
		"event":    "phx_join",
		"payload":  map[string]any{"join_ref": "1"},
	}
	if err := wsjson.Write(ctx, ws, joinMsg); err != nil {
		t.Fatalf("Failed to send join: %v", err)
	}

	// 4. Read join reply
	var reply map[string]any
	if err := wsjson.Read(ctx, ws, &reply); err != nil {
		t.Fatalf("Failed to read join reply: %v", err)
	}

	t.Logf("✓ Received reply: event=%v", reply["event"])

	// 5. Send navigation event
	topic := reply["topic"]
	if topic == nil {
		topic = "lv:docs"
	}

	navMsg := map[string]any{
		"ref":     "2",
		"topic":   topic,
		"event":   "nav",
		"payload": map[string]any{"section": "core-concepts"},
	}
	if err := wsjson.Write(ctx, ws, navMsg); err != nil {
		t.Fatalf("Failed to send nav event: %v", err)
	}

	t.Log("✓ Sent nav event for 'core-concepts'")

	// 6. Read diff response
	var diff map[string]any
	if err := wsjson.Read(ctx, ws, &diff); err != nil {
		t.Fatalf("Failed to read diff: %v", err)
	}

	t.Logf("✓ Received message: event=%v", diff["event"])

	// Check if it's a diff with slots
	if diff["event"] == "diff" {
		payload := diff["payload"].(map[string]any)
		t.Logf("  Diff payload keys: %v", getKeys(payload))

		// Check for slots
		if s, ok := payload["s"]; ok && s != nil {
			t.Logf("  Text slots (s): %v", getKeys(s.(map[string]any)))
		}
		if h, ok := payload["h"]; ok && h != nil {
			hSlots := h.(map[string]any)
			t.Logf("  HTML slots (h): %v", getKeys(hSlots))

			// Check if content slot has Core Concepts
			if content, ok := hSlots["content"].(string); ok {
				if strings.Contains(content, "Core Concepts") {
					t.Log("✓ Content slot contains 'Core Concepts'")
				} else {
					t.Error("✗ Content slot doesn't contain 'Core Concepts'")
				}
			}
			if sidebar, ok := hSlots["sidebar"].(string); ok {
				if strings.Contains(sidebar, "docs-nav-item-active") {
					t.Log("✓ Sidebar has active nav item")
				}
			}
		}
		if f, ok := payload["f"]; ok && f != nil && f != "" {
			t.Logf("  Full render (f): %d bytes", len(f.(string)))
		}
	}

	// Pretty print the diff for debugging
	diffJSON, _ := json.MarshalIndent(diff, "", "  ")
	if len(diffJSON) > 500 {
		t.Logf("Diff (truncated):\n%s...", string(diffJSON[:500]))
	} else {
		t.Logf("Diff:\n%s", string(diffJSON))
	}
}

func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
