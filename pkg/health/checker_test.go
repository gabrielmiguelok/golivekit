package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthCheck_AllPass(t *testing.T) {
	hc := NewChecker()
	hc.SetVersion("1.0.0")

	hc.AddCheck("ping", func(ctx context.Context) error {
		return nil
	}, time.Second)

	hc.AddCheck("database", func(ctx context.Context) error {
		return nil
	}, time.Second)

	status := hc.Check(context.Background())

	if status.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %s", status.Status)
	}

	if len(status.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(status.Checks))
	}

	for name, result := range status.Checks {
		if result.Status != StatusHealthy {
			t.Errorf("Check %s should be healthy", name)
		}
		if result.Error != "" {
			t.Errorf("Check %s should have no error", name)
		}
	}

	if status.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", status.Version)
	}
}

func TestHealthCheck_OneFails(t *testing.T) {
	hc := NewChecker()

	hc.AddCheck("passing", func(ctx context.Context) error {
		return nil
	}, time.Second)

	hc.AddCheck("failing", func(ctx context.Context) error {
		return errors.New("database connection failed")
	}, time.Second)

	status := hc.Check(context.Background())

	// Non-critical failure = degraded
	if status.Status != StatusDegraded {
		t.Errorf("Expected degraded, got %s", status.Status)
	}

	if status.Checks["passing"].Status != StatusHealthy {
		t.Error("Passing check should be healthy")
	}

	if status.Checks["failing"].Status != StatusUnhealthy {
		t.Error("Failing check should be unhealthy")
	}

	if status.Checks["failing"].Error == "" {
		t.Error("Failing check should have error message")
	}
}

func TestHealthCheck_CriticalFails(t *testing.T) {
	hc := NewChecker()

	hc.AddCheck("passing", func(ctx context.Context) error {
		return nil
	}, time.Second)

	hc.AddCriticalCheck("critical-fail", func(ctx context.Context) error {
		return errors.New("critical service down")
	}, time.Second)

	status := hc.Check(context.Background())

	// Critical failure = unhealthy
	if status.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy, got %s", status.Status)
	}
}

func TestHealthCheck_Timeout(t *testing.T) {
	hc := NewChecker()

	hc.AddCheck("slow", func(ctx context.Context) error {
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}, 50*time.Millisecond) // Short timeout

	status := hc.Check(context.Background())

	if status.Checks["slow"].Status != StatusUnhealthy {
		t.Error("Timed out check should be unhealthy")
	}
}

func TestHealthCheck_LivenessHandler(t *testing.T) {
	hc := NewChecker()
	handler := hc.LivenessHandler()

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "alive" {
		t.Error("Expected status 'alive'")
	}
}

func TestHealthCheck_ReadinessHandler_Healthy(t *testing.T) {
	hc := NewChecker()
	hc.AddCriticalCheck("db", func(ctx context.Context) error {
		return nil
	}, time.Second)

	handler := hc.ReadinessHandler()

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestHealthCheck_ReadinessHandler_Unhealthy(t *testing.T) {
	hc := NewChecker()
	hc.AddCriticalCheck("db", func(ctx context.Context) error {
		return errors.New("connection refused")
	}, time.Second)

	handler := hc.ReadinessHandler()

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503, got %d", w.Code)
	}
}

func TestHealthCheck_FullHandler(t *testing.T) {
	hc := NewChecker()
	hc.SetVersion("2.0.0")
	hc.AddCheck("cache", func(ctx context.Context) error {
		return nil
	}, time.Second)

	handler := hc.HealthHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Always returns 200
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var status HealthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if status.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", status.Version)
	}

	if _, ok := status.Checks["cache"]; !ok {
		t.Error("Expected cache check in response")
	}
}

func TestDefaultChecker(t *testing.T) {
	hc := DefaultChecker("1.0.0")

	status := hc.Check(context.Background())

	if status.Status != StatusHealthy {
		t.Error("Default checker should be healthy")
	}

	if status.Version != "1.0.0" {
		t.Error("Version should be set")
	}

	if _, ok := status.Checks["ping"]; !ok {
		t.Error("Default checker should have ping check")
	}
}

func TestWebSocketPoolCheck(t *testing.T) {
	current := 50
	max := 100

	check := WebSocketPoolCheck(func() int { return current }, max)

	// Under capacity
	err := check(context.Background())
	if err != nil {
		t.Errorf("Should pass when under capacity: %v", err)
	}

	// At capacity
	current = 100
	err = check(context.Background())
	if err == nil {
		t.Error("Should fail when at capacity")
	}
}
