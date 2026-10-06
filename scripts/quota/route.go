package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// The proxy serves each request from the highest-priority accounts that are not cooling down, so a
// boosted account is drained first and the others take over as soon as it hits a limit.
const (
	normalPriority = 0
	// A boosted account gets boostPriority minus the hours until its weekly reset: sooner is higher.
	boostPriority = 100
	// A failed run (the proxy may still be starting) is retried this soon instead of a full interval.
	retryAfter = time.Minute
)

// routeEvery runs route once, or forever at the given interval.
func routeEvery(out io.Writer, p proxy, every time.Duration) error {
	logger := log.New(out, "quota: ", log.LstdFlags)
	for {
		err := route(logger.Printf, p, time.Now())
		if every <= 0 {
			return err
		}
		wait := every
		if err != nil {
			logger.Printf("routing failed, retrying in %s: %v", retryAfter, err)
			wait = retryAfter
		}
		time.Sleep(wait)
	}
}

// route sets the priority of every enabled Claude and Codex account: boosted while its weekly
// capacity is about to expire unused, normal otherwise. Accounts whose usage could not be read keep
// their current priority.
func route(logf func(string, ...any), p proxy, now time.Time) error {
	usages, err := inspect(p, now)
	if err != nil {
		return err
	}
	checked, boosted, changed := 0, 0, 0
	for _, usage := range usages {
		if !usage.Supported || usage.Disabled {
			continue
		}
		checked++
		if usage.Err != nil {
			logf("%s: %v; priority stays %d", usage.Name, usage.Err, usage.Priority)
			continue
		}
		want, reason := targetPriority(usage.Windows, now)
		if want != normalPriority {
			boosted++
		}
		if want == usage.Priority {
			continue
		}
		patch := map[string]any{"name": usage.Name, "priority": want}
		if err := p.call(http.MethodPatch, "auth-files/fields", patch, nil); err != nil {
			logf("%s: setting priority %d failed: %v", usage.Name, want, err)
			continue
		}
		changed++
		logf("%s: priority %d -> %d (%s)", usage.Name, usage.Priority, want, reason)
	}
	logf("checked %d accounts: %d boosted, %d changed", checked, boosted, changed)
	return nil
}

// targetPriority boosts an account by its soonest weekly window expiring with capacity left.
func targetPriority(windows []window, now time.Time) (int, string) {
	var soonest *window
	for i, w := range windows {
		if isExpiring(w, now) && (soonest == nil || w.Reset.Before(*soonest.Reset)) {
			soonest = &windows[i]
		}
	}
	if soonest == nil {
		return normalPriority, "no weekly capacity expiring"
	}
	priority := boostPriority - int(soonest.Reset.Sub(now).Hours())
	return priority, fmt.Sprintf("%s: %.0f%% left, resets in %s", soonest.Label, 100-soonest.Used, until(*soonest.Reset, now))
}
