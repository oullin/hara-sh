package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// A weekly window resetting this soon with capacity left is reported as expiring.
const expiringWithin = 24 * time.Hour

// show prints every account's windows, then the weekly capacity about to expire unused.
func show(out io.Writer, p proxy, now time.Time) error {
	usages, err := inspect(p, now)
	if err != nil {
		return err
	}
	var expiring []string
	for _, usage := range usages {
		fmt.Fprintf(out, "%-8s %s", usage.Provider, usage.Name)
		if usage.Priority != 0 {
			fmt.Fprintf(out, "  (priority %d)", usage.Priority)
		}
		fmt.Fprintln(out)
		if usage.Disabled {
			fmt.Fprintln(out, "    disabled in the proxy")
		} else if usage.NextRetryAfter != nil && usage.NextRetryAfter.After(now) {
			fmt.Fprintf(out, "    proxy cooldown until %s (in %s)\n", clock(*usage.NextRetryAfter), until(*usage.NextRetryAfter, now))
		}
		switch {
		case !usage.Supported:
			fmt.Fprintln(out, "    usage not available for this provider")
		case usage.Err != nil:
			fmt.Fprintf(out, "    %v\n", usage.Err)
		default:
			expiring = append(expiring, report(out, usage.Name, usage.Windows, now)...)
		}
	}
	if len(expiring) > 0 {
		fmt.Fprintf(out, "\nExpiring unused within %dh:\n", int(expiringWithin.Hours()))
		for _, line := range expiring {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
	return nil
}

// report prints one account's windows and returns the weekly ones expiring with capacity left.
func report(out io.Writer, name string, windows []window, now time.Time) []string {
	if len(windows) == 0 {
		fmt.Fprintln(out, "    no usage windows in the response")
	}
	var expiring []string
	for _, w := range windows {
		when := "no active window (starts on first use)"
		if w.Reset != nil {
			when = fmt.Sprintf("resets %s (in %s)", clock(*w.Reset), until(*w.Reset, now))
		}
		fmt.Fprintf(out, "    %-14s %5.1f%% used  %5.1f%% left  %s\n", w.Label, w.Used, max(0, 100-w.Used), when)
		if isExpiring(w, now) {
			expiring = append(expiring, fmt.Sprintf("%s %s: %.0f%% left, resets in %s", name, w.Label, 100-w.Used, until(*w.Reset, now)))
		}
	}
	return expiring
}

func isExpiring(w window, now time.Time) bool {
	weekly := false
	for _, field := range strings.Fields(w.Label) {
		weekly = weekly || field == "7d"
	}
	return weekly && w.Reset != nil && w.Used < 100 && w.Reset.After(now) && w.Reset.Sub(now) <= expiringWithin
}

func clock(t time.Time) string {
	return t.Local().Format("Mon 02 Jan 15:04")
}

func until(t, now time.Time) string {
	d := max(0, t.Sub(now))
	days := int(d / (24 * time.Hour))
	hours := int(d/time.Hour) % 24
	minutes := int(d/time.Minute) % 60
	if days > 0 {
		return fmt.Sprintf("%dd%02dh", days, hours)
	}
	return fmt.Sprintf("%dh%02dm", hours, minutes)
}
