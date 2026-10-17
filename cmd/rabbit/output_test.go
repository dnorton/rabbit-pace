package main

import (
	"bytes"
	"io"
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
	got := runOutput(t, colorprofile.TrueColor, "-pace", "4:00", "-dist", "3k", "-halfway", "-mph", "-md")
	want := `- **Distance:** 3 km · 1.86 mi
- **Time:** **12:00**
- **Pace:** 4:00 /km · 6:26 /mi
- **Speed:** 9.3 mph

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
	got := runOutput(t, colorprofile.TrueColor, "-time", "45:30", "-dist", "10k", "-md", "-only")
	want := `- **Distance:** 10 km · 6.21 mi
- **Time:** 45:30
- **Pace:** **4:33 /km · 7:19 /mi**
`
	if got != want {
		t.Errorf("markdown output:\n%s\nwant:\n%s", got, want)
	}
}

func TestStyledWithoutColor(t *testing.T) {
	// NoTTY is what piped output gets: the table, with no escape codes.
	got := runOutput(t, colorprofile.NoTTY, "-pace", "4:00", "-dist", "2k", "-neg", "5")
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
	got := runOutput(t, colorprofile.ANSI, "-pace", "4:00", "-dist", "4k", "-neg", "5")
	if !strings.Contains(got, green) || !strings.Contains(got, yellow) {
		t.Errorf("negative split output should use green and yellow:\n%q", got)
	}

	// Even pace: every split is on target, so no trend colours.
	got = runOutput(t, colorprofile.ANSI, "-pace", "4:00", "-dist", "4k")
	if strings.Contains(got, green) || strings.Contains(got, yellow) {
		t.Errorf("even pace output should not use trend colours:\n%q", got)
	}
}

func TestPaceAnyTwo(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string // the calculated line, in bold
	}{
		{"pace from time and distance", []string{"-time", "45:30", "-dist", "10k"}, "- **Pace:** **4:33 /km · 7:19 /mi**"},
		{"time from pace and distance", []string{"-pace", "7:00", "-unit", "mi", "-dist", "half"}, "- **Time:** **1:31:46**"},
		{"distance from time and pace", []string{"-pace", "5:00", "-time", "30:00"}, "- **Distance:** **6 km · 3.73 mi**"},
		{"bare distance uses -unit", []string{"-pace", "7:00", "-unit", "mi", "-dist", "10"}, "- **Time:** **1:10:00**"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-md", "-only"}, tt.args...)
			got := runOutput(t, colorprofile.NoTTY, args...)
			if !strings.Contains(got, tt.want+"\n") {
				t.Errorf("output missing %q:\n%s", tt.want, got)
			}
			if n := strings.Count(got, ":** **"); n != 1 {
				t.Errorf("want exactly one calculated value, got %d:\n%s", n, got)
			}
		})
	}

	for _, args := range [][]string{
		{"-dist", "10k"},
		{"-time", "1:00:00", "-pace", "5:00", "-dist", "10k"},
	} {
		err := run(args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "exactly two") {
			t.Errorf("run(%q) error = %v, want 'exactly two'", args, err)
		}
	}
}
