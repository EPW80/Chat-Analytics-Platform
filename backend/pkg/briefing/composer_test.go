package briefing

import (
	"strings"
	"testing"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

func anomaly(kind analytics.AnomalyKind, observed, baseline float64, sev analytics.Severity) analytics.Anomaly {
	return analytics.Anomaly{
		Kind:     kind,
		Scope:    "global",
		Observed: observed,
		Baseline: baseline,
		Severity: sev,
		At:       time.Now(),
	}
}

func TestCompose(t *testing.T) {
	cases := []struct {
		name     string
		anomaly  analytics.Anomaly
		contains []string
	}{
		{
			name:     "rate spike",
			anomaly:  anomaly(analytics.RateSpike, 400, 90, analytics.Yellow),
			contains: []string{"Message-rate spike", "four hundred", "ninety", "Monitoring."},
		},
		{
			name:     "latency breach",
			anomaly:  anomaly(analytics.LatencyBreach, 200, 150, analytics.Red),
			contains: []string{"latency breach", "two hundred milliseconds", "Escalating."},
		},
		{
			name:     "connection surge",
			anomaly:  anomaly(analytics.ConnectionSurge, 340, 12, analytics.Yellow),
			contains: []string{"Connection surge", "three hundred forty", "twelve"},
		},
		{
			name:     "traffic dropout",
			anomaly:  anomaly(analytics.TrafficDropout, 45, 0, analytics.Yellow),
			contains: []string{"Traffic dropout", "gone silent", "forty-five"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compose(tc.anomaly)
			if got == "" {
				t.Fatal("Compose returned empty script")
			}
			if len(got) > maxScriptLen {
				t.Fatalf("script %d chars exceeds cap %d", len(got), maxScriptLen)
			}
			if !strings.HasPrefix(got, "Ops, analytics.") {
				t.Errorf("script should open in control-room register, got %q", got)
			}
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("script missing %q\ngot: %s", want, got)
				}
			}
			assertSpokenNumbers(t, got)
		})
	}
}

// assertSpokenNumbers guards the one thing TTS cannot recover from: digits and
// unit abbreviations, which get read out as letters or skipped entirely.
func assertSpokenNumbers(t *testing.T, script string) {
	t.Helper()
	if strings.Contains(script, "ms") {
		t.Errorf("found abbreviated unit 'ms', numbers must be spelled for speech: %s", script)
	}
	for _, r := range script {
		if r >= '0' && r <= '9' {
			t.Errorf("found digit %q, numbers must be spelled for speech: %s", r, script)
			return
		}
	}
}

func TestCompose_IsPure(t *testing.T) {
	a := anomaly(analytics.RateSpike, 400, 90, analytics.Yellow)
	if first, second := Compose(a), Compose(a); first != second {
		t.Errorf("Compose is not deterministic:\n%q\n%q", first, second)
	}
}

func TestCompose_UnknownKindStillSpeaks(t *testing.T) {
	got := Compose(anomaly(analytics.AnomalyKind("meteor_strike"), 1, 0, analytics.Yellow))
	if got == "" {
		t.Fatal("unknown kind produced no script; the fallback must still say something")
	}
	if !strings.Contains(got, "Unclassified anomaly") {
		t.Errorf("want the unclassified fallback, got %q", got)
	}
}

func TestCompose_TruncatesOverflow(t *testing.T) {
	// A scope long enough to blow the cap on its own.
	a := anomaly(analytics.RateSpike, 400, 90, analytics.Yellow)
	a.Scope = strings.Repeat("verylongroomname", 60)

	got := Compose(a)
	if len(got) > maxScriptLen {
		t.Fatalf("script %d chars exceeds cap %d", len(got), maxScriptLen)
	}
	if !strings.HasSuffix(got, overflowSuffix) {
		t.Errorf("truncated script should hand off to the dashboard, got tail %q", got[len(got)-40:])
	}
}

func TestSpeakInt(t *testing.T) {
	cases := map[int64]string{
		0:       "zero",
		7:       "seven",
		12:      "twelve",
		19:      "nineteen",
		20:      "twenty",
		45:      "forty-five",
		90:      "ninety",
		100:     "one hundred",
		400:     "four hundred",
		512:     "five hundred twelve",
		1000:    "one thousand",
		1250:    "one thousand two hundred fifty",
		999999:  "nine hundred ninety-nine thousand nine hundred ninety-nine",
		1000000: "1000000", // past the spoken range, digits are the honest fallback
	}
	for in, want := range cases {
		if got := speakInt(in); got != want {
			t.Errorf("speakInt(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestSpeakMillis(t *testing.T) {
	cases := map[float64]string{
		0:     "one millisecond", // never announce "zero milliseconds"
		0.4:   "one millisecond",
		1:     "one millisecond",
		11.6:  "twelve milliseconds",
		200:   "two hundred milliseconds",
		149.5: "one hundred fifty milliseconds",
	}
	for in, want := range cases {
		if got := speakMillis(in); got != want {
			t.Errorf("speakMillis(%v) = %q, want %q", in, got, want)
		}
	}
}
