package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// runOutput runs rabbit with args and returns what it printed, with the
// given colour profile applied as it would be for a real terminal.
func runOutput(t *testing.T, profile colorprofile.Profile, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	w := &colorprofile.Writer{Forward: &buf, Profile: profile}
	if err := run(args, w); err != nil {
		t.Fatalf("run(%q) error: %v", args, err)
	}
	return buf.String()
}

func TestSplitsMarkdown(t *testing.T) {
	got := runOutput(t, colorprofile.TrueColor, "splits", "-pace", "4:00", "-dist", "3k", "-halfway", "-mph", "-md")
	want := `- **Distance:** 3 km · 1.86 mi
- **Target pace:** 4:00 /km · 6:26 /mi
- **Speed:** 9.3 mph
- **Finish:** 12:00

| Split | Distance (km) | Pace /km | Pace /mi | MPH | Split Time | Total Time |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 1 | 4:00 | 6:26 | 9.3 | 4:00 | 4:00 |
| **Half** | 1.5 | 4:00 | 6:26 | 9.3 |  | 6:00 |
| 2 | 2 | 4:00 | 6:26 | 9.3 | 4:00 | 8:00 |
| 3 | 3 | 4:00 | 6:26 | 9.3 | 4:00 | 12:00 |
`
	if got != want {
		t.Errorf("markdown output:\n%s\nwant:\n%s", got, want)
	}
}

func TestPaceMarkdown(t *testing.T) {
	got := runOutput(t, colorprofile.TrueColor, "pace", "-time", "45:30", "-dist", "10k", "-md")
	want := `- **Distance:** 10 km · 6.21 mi
- **Time:** 45:30
- **Pace:** 4:33 /km · 7:19 /mi
`
	if got != want {
		t.Errorf("markdown output:\n%s\nwant:\n%s", got, want)
	}
}

func TestStyledWithoutColor(t *testing.T) {
	// NoTTY is what piped output gets: the table, with no escape codes.
	got := runOutput(t, colorprofile.NoTTY, "splits", "-pace", "4:00", "-dist", "2k", "-neg", "5")
	if strings.Contains(got, "\x1b[") {
		t.Errorf("piped output contains escape codes:\n%q", got)
	}
	for _, want := range []string{"Negative split 5%", "╭", "│ Split │", "4:12", "3:48", "8:00"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestStyledTrendColors(t *testing.T) {
	const green, yellow = "\x1b[32m", "\x1b[33m"

	// A negative split starts slower (yellow) and finishes faster (green).
	got := runOutput(t, colorprofile.ANSI, "splits", "-pace", "4:00", "-dist", "4k", "-neg", "5")
	if !strings.Contains(got, green) || !strings.Contains(got, yellow) {
		t.Errorf("negative split output should use green and yellow:\n%q", got)
	}

	// Even pace: every split is on target, so no trend colours.
	got = runOutput(t, colorprofile.ANSI, "splits", "-pace", "4:00", "-dist", "4k")
	if strings.Contains(got, green) || strings.Contains(got, yellow) {
		t.Errorf("even pace output should not use trend colours:\n%q", got)
	}
}
