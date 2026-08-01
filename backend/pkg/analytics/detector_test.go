package analytics

import (
	"testing"
	"time"
)

// mkMetrics builds a Metrics snapshot with a given per-minute window (oldest first)
// and P99 latency. Helper for table-driven detector tests.
func mkMetrics(perMin []int64, p99 float64, activeConns int64) Metrics {
	return Metrics{
		MessagesPerMinute: perMin,
		LatencyP99Ms:      p99,
		ActiveConnections: activeConns,
	}
}

// window builds a 15-slot MessagesPerMinute window: 14 baseline slots at `base`,
// current minute at `cur`.
func window(base, cur int64) []int64 {
	w := make([]int64, 15)
	for i := 0; i < 14; i++ {
		w[i] = base
	}
	w[14] = cur
	return w
}

// kinds extracts the AnomalyKinds from a result for order-independent comparison.
func kinds(as []Anomaly) map[AnomalyKind]Anomaly {
	m := make(map[AnomalyKind]Anomaly, len(as))
	for _, a := range as {
		m[a.Kind] = a
	}
	return m
}

func assertKinds(t *testing.T, got []Anomaly, want []AnomalyKind) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d anomalies %v, want %d %v", len(got), got, len(want), want)
	}
	byKind := kinds(got)
	if len(byKind) != len(got) {
		t.Fatalf("duplicate kinds in result: %v", got)
	}
	for _, k := range want {
		if _, ok := byKind[k]; !ok {
			t.Errorf("missing kind %q in %v", k, got)
		}
	}
}

func TestDetector_Check(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name string
		cur  Metrics
		want []AnomalyKind
	}{
		{
			// 400 vs baseline 90: >3x and over the 50 floor.
			name: "rate spike over floor",
			cur:  mkMetrics(window(90, 400), 12, 10),
			want: []AnomalyKind{RateSpike},
		},
		{
			// 3 messages on a dead server is an infinite multiple of a zero
			// baseline — the floor is the only thing stopping a briefing here.
			name: "idle server no spike",
			cur:  mkMetrics(window(0, 3), 5, 1),
			want: nil,
		},
		{
			// Over the floor but only 2x baseline — under SpikeMultiplier.
			name: "growth below multiplier",
			cur:  mkMetrics(window(100, 200), 12, 10),
			want: nil,
		},
		{
			name: "p99 red breach",
			cur:  mkMetrics(make([]int64, 15), 200, 10),
			want: []AnomalyKind{LatencyBreach},
		},
		{
			name: "p99 yellow breach",
			cur:  mkMetrics(make([]int64, 15), 60, 10),
			want: []AnomalyKind{LatencyBreach},
		},
		{
			name: "p99 under yellow is quiet",
			cur:  mkMetrics(make([]int64, 15), 49, 10),
			want: nil,
		},
		{
			name: "spike and latency breach together",
			cur:  mkMetrics(window(90, 400), 200, 10),
			want: []AnomalyKind{RateSpike, LatencyBreach},
		},
		{
			name: "short window cannot spike",
			cur:  mkMetrics([]int64{9000}, 5, 1),
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDetector(DefaultThresholds())
			assertKinds(t, d.Check(tc.cur, now), tc.want)
		})
	}
}

func TestDetector_SeverityBands(t *testing.T) {
	now := time.Now()
	th := DefaultThresholds()

	t.Run("latency yellow then red", func(t *testing.T) {
		d := NewDetector(th)
		got := d.Check(mkMetrics(make([]int64, 15), th.LatencyYellowMs+1, 0), now)
		if len(got) != 1 || got[0].Severity != Yellow {
			t.Fatalf("want one Yellow, got %v", got)
		}
		if got[0].Baseline != th.LatencyYellowMs {
			t.Errorf("Baseline = %v, want the yellow threshold %v", got[0].Baseline, th.LatencyYellowMs)
		}

		// Past the debounce window so the second breach is allowed to fire.
		later := now.Add(th.DebounceWindow + time.Second)
		got = d.Check(mkMetrics(make([]int64, 15), th.LatencyRedMs+1, 0), later)
		if len(got) != 1 || got[0].Severity != Red {
			t.Fatalf("want one Red, got %v", got)
		}
		if got[0].Baseline != th.LatencyRedMs {
			t.Errorf("Baseline = %v, want the red threshold %v", got[0].Baseline, th.LatencyRedMs)
		}
	})

	t.Run("rate spike red past double multiplier", func(t *testing.T) {
		d := NewDetector(th)
		// baseline 90, multiplier 3 => Red above 540.
		got := d.Check(mkMetrics(window(90, 600), 0, 0), now)
		if len(got) != 1 || got[0].Severity != Red {
			t.Fatalf("want one Red rate spike, got %v", got)
		}
		if got[0].Observed != 600 || got[0].Baseline != 90 {
			t.Errorf("Observed/Baseline = %v/%v, want 600/90", got[0].Observed, got[0].Baseline)
		}
	})
}

func TestDetector_ConnectionSurge(t *testing.T) {
	now := time.Now()
	th := DefaultThresholds()
	d := NewDetector(th)

	// First Check only establishes prev — a delta needs two snapshots.
	if got := d.Check(mkMetrics(make([]int64, 15), 0, 10), now); len(got) != 0 {
		t.Fatalf("first Check should report nothing, got %v", got)
	}

	got := d.Check(mkMetrics(make([]int64, 15), 0, 10+th.ConnSurgeDelta+1), now.Add(time.Second))
	assertKinds(t, got, []AnomalyKind{ConnectionSurge})
	if got[0].Baseline != 10 {
		t.Errorf("Baseline = %v, want the previous connection count 10", got[0].Baseline)
	}

	// A delta at exactly the threshold is not a surge.
	d2 := NewDetector(th)
	d2.Check(mkMetrics(make([]int64, 15), 0, 0), now)
	if got := d2.Check(mkMetrics(make([]int64, 15), 0, th.ConnSurgeDelta), now.Add(time.Second)); len(got) != 0 {
		t.Fatalf("delta at threshold should not fire, got %v", got)
	}
}

func TestDetector_TrafficDropout(t *testing.T) {
	now := time.Now()
	d := NewDetector(DefaultThresholds())

	// Busy, then silent.
	d.Check(mkMetrics(window(40, 40), 5, 10), now)
	got := d.Check(mkMetrics(window(40, 0), 5, 10), now.Add(time.Minute))
	assertKinds(t, got, []AnomalyKind{TrafficDropout})
	if got[0].Observed != 40 {
		t.Errorf("Observed = %v, want the previous rate 40", got[0].Observed)
	}

	// Already silent stays silent — a quiet server is not an event.
	quiet := NewDetector(DefaultThresholds())
	quiet.Check(mkMetrics(window(0, 0), 5, 10), now)
	if got := quiet.Check(mkMetrics(window(0, 0), 5, 10), now.Add(time.Minute)); len(got) != 0 {
		t.Fatalf("idle-to-idle should not fire, got %v", got)
	}
}

func TestDetector_Debounce(t *testing.T) {
	now := time.Now()
	th := DefaultThresholds()
	d := NewDetector(th)
	spike := mkMetrics(window(90, 400), 0, 0)

	if got := d.Check(spike, now); len(got) != 1 {
		t.Fatalf("first spike should fire, got %v", got)
	}

	// Same condition still tripping, well inside the window.
	for _, offset := range []time.Duration{time.Second, time.Minute, th.DebounceWindow - time.Second} {
		if got := d.Check(spike, now.Add(offset)); len(got) != 0 {
			t.Errorf("spike at +%v should be debounced, got %v", offset, got)
		}
	}

	if got := d.Check(spike, now.Add(th.DebounceWindow)); len(got) != 1 {
		t.Fatalf("spike at the window edge should fire again, got %v", got)
	}
}

func TestDetector_DebounceIsPerKind(t *testing.T) {
	now := time.Now()
	d := NewDetector(DefaultThresholds())

	// A rate spike must not suppress an unrelated latency breach.
	if got := d.Check(mkMetrics(window(90, 400), 0, 0), now); len(got) != 1 {
		t.Fatalf("want the rate spike, got %v", got)
	}
	got := d.Check(mkMetrics(window(90, 400), 200, 0), now.Add(time.Second))
	assertKinds(t, got, []AnomalyKind{LatencyBreach})
}

// The detector must not retain the caller's slice: Tracker reuses its window
// buffer, so a retained reference would let a later tick rewrite our baseline.
func TestDetector_CopiesWindow(t *testing.T) {
	now := time.Now()
	d := NewDetector(DefaultThresholds())

	w := window(40, 40)
	d.Check(mkMetrics(w, 5, 10), now)
	for i := range w {
		w[i] = 0 // caller reuses the buffer
	}

	got := d.Check(mkMetrics(window(40, 0), 5, 10), now.Add(time.Minute))
	assertKinds(t, got, []AnomalyKind{TrafficDropout})
}
