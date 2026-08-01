package briefing

import (
	"fmt"
	"strings"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

// maxScriptLen caps a script for cost and listener attention. Overflow is
// truncated with a pointer to the dashboard.
const maxScriptLen = 600

// overflowSuffix ends a truncated script. Scripts are read aloud, so handing off
// to the screen beats a sentence that stops mid-thought.
const overflowSuffix = " Full detail on the dashboard."

// Compose renders an anomaly as a control-room script. Pure function: same
// anomaly in, same string out. No LLM — templates only (see BUILD_PLAN §Phase 1b).
// The returned string is the text-only fallback artifact when TTS is unavailable.
//
// Register: short declarative sentences, numbers spelled for speech
// ("twelve milliseconds", not "12ms"). Example:
//
//	"Ops, analytics. Message-rate spike in the global room — four hundred per
//	 minute against a baseline of ninety. Monitoring."
func Compose(a analytics.Anomaly) string {
	var body string

	// Clauses join with an em-dash rather than a full stop: read aloud, a
	// sentence that opens on a spelled-out number lands as a fragment.
	switch a.Kind {
	case analytics.RateSpike:
		body = fmt.Sprintf(
			"Message-rate spike in the %s room — %s messages per minute against a baseline of %s.",
			spokenScope(a.Scope), speakCount(a.Observed), speakCount(a.Baseline),
		)
	case analytics.LatencyBreach:
		body = fmt.Sprintf(
			"Broadcast latency breach in the %s room — P ninety-nine at %s, past the %s millisecond ceiling.",
			spokenScope(a.Scope), speakMillis(a.Observed), speakCount(a.Baseline),
		)
	case analytics.ConnectionSurge:
		body = fmt.Sprintf(
			"Connection surge in the %s room — %s clients connected, up from %s.",
			spokenScope(a.Scope), speakCount(a.Observed), speakCount(a.Baseline),
		)
	case analytics.TrafficDropout:
		body = fmt.Sprintf(
			"Traffic dropout in the %s room — chat has gone silent after %s messages per minute.",
			spokenScope(a.Scope), speakCount(a.Observed),
		)
	default:
		// Unknown kind: say something useful rather than going mute.
		body = fmt.Sprintf("Unclassified anomaly in the %s room.", spokenScope(a.Scope))
	}

	return truncate("Ops, analytics. " + body + " " + closer(a.Severity))
}

// closer signs off in the register the severity warrants. Yellow observes; Red
// asks for eyes on it.
func closer(s analytics.Severity) string {
	if s == analytics.Red {
		return "Escalating. Eyes on the dashboard."
	}
	return "Monitoring."
}

// spokenScope renders a scope for speech. "global" is the v1 scope; room IDs
// read as-is once per-room detection lands.
func spokenScope(scope string) string {
	if scope == "" {
		return "global"
	}
	return scope
}

// truncate enforces maxScriptLen without cutting mid-word, handing the listener
// off to the dashboard instead.
func truncate(script string) string {
	if len(script) <= maxScriptLen {
		return script
	}
	keep := maxScriptLen - len(overflowSuffix)
	if keep < 1 {
		return script[:maxScriptLen]
	}
	cut := script[:keep]
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:-") + "." + overflowSuffix
}

// speakMillis renders a millisecond duration for speech: "twelve milliseconds",
// never "12ms". Sub-millisecond values round up to one — "zero milliseconds"
// sounds like a bug being read aloud.
func speakMillis(ms float64) string {
	n := int64(ms + 0.5)
	if n < 1 {
		n = 1
	}
	unit := "milliseconds"
	if n == 1 {
		unit = "millisecond"
	}
	return speakInt(n) + " " + unit
}

// speakCount renders a count for speech, rounding to the nearest whole number.
func speakCount(v float64) string {
	n := int64(v + 0.5)
	if n < 0 {
		n = 0
	}
	return speakInt(n)
}

var (
	ones = []string{
		"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
		"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen",
		"seventeen", "eighteen", "nineteen",
	}
	tens = []string{
		"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety",
	}
)

// speakInt spells a non-negative integer in English words. At a million and above
// it falls back to digits: a briefing that reads a nine-digit number aloud is
// worse than one that points at the screen, and metrics that large mean the
// dashboard is already the better surface.
func speakInt(n int64) string {
	switch {
	case n < 0:
		return fmt.Sprintf("%d", n)
	case n < 20:
		return ones[n]
	case n < 100:
		word := tens[n/10]
		if r := n % 10; r != 0 {
			word += "-" + ones[r]
		}
		return word
	case n < 1000:
		word := ones[n/100] + " hundred"
		if r := n % 100; r != 0 {
			word += " " + speakInt(r)
		}
		return word
	case n < 1000000:
		word := speakInt(n/1000) + " thousand"
		if r := n % 1000; r != 0 {
			word += " " + speakInt(r)
		}
		return word
	default:
		return fmt.Sprintf("%d", n)
	}
}
