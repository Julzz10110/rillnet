package monitoring

import (
	"context"
	"sync"
	"time"
)

type HealthChecker struct {
	checks []HealthCheck
	mu     sync.RWMutex
}

type HealthCheck struct {
	Name     string
	Check    func(ctx context.Context) (bool, error)
	Interval time.Duration
	Timeout  time.Duration
}

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make([]HealthCheck, 0),
	}
}

func (h *HealthChecker) AddCheck(name string, check func(ctx context.Context) (bool, error), interval, timeout time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.checks = append(h.checks, HealthCheck{
		Name:     name,
		Check:    check,
		Interval: interval,
		Timeout:  timeout,
	})
}

func (h *HealthChecker) CheckAll(ctx context.Context) HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Checks:    make(map[string]string),
	}

	for _, check := range h.checks {
		checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
		defer cancel()

		healthy, err := check.Check(checkCtx)
		if err != nil || !healthy {
			status.Status = "unhealthy"
			if err != nil {
				status.Checks[check.Name] = err.Error()
			} else {
				status.Checks[check.Name] = "check failed"
			}
		} else {
			status.Checks[check.Name] = "healthy"
		}
	}

	return status
}

func (h *HealthChecker) StartBackgroundChecks(ctx context.Context) {
	for _, check := range h.checks {
		go h.runCheckPeriodically(ctx, check)
	}
}

func (h *HealthChecker) runCheckPeriodically(ctx context.Context, check HealthCheck) {
	ticker := time.NewTicker(check.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
			_, _ = check.Check(checkCtx)
			cancel()
		}
	}
}
