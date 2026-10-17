package main

import (
	"io"
	"strings"
	"testing"
)

func TestRunRejectsExtraArgs(t *testing.T) {
	tests := [][]string{
		{"-pace", "4:30", "-dist", "10k", "-every", "2", "miles"},
		{"-pace", "4:30", "-dist", "10k", "oops", "-mph"},
		{"-time", "45:30", "-dist", "13.1", "miles", "-only"},
	}
	for _, args := range tests {
		err := run(args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
			t.Errorf("run(%q) error = %v, want unexpected argument", args, err)
		}
	}
}

func TestRunAcceptsValidArgs(t *testing.T) {
	tests := [][]string{
		{"-pace", "4:30", "-dist", "10k", "-every", "2 miles"},
		{"-pace", "4:30", "-dist", "10k", "-every", "2mi", "-mph"},
		{"-time", "45:30", "-dist", "10k", "-only"},
	}
	for _, args := range tests {
		if err := run(args, io.Discard); err != nil {
			t.Errorf("run(%q) error: %v", args, err)
		}
	}
}

func TestRunRemovedSubcommands(t *testing.T) {
	tests := []struct {
		args []string
		hint string
	}{
		{[]string{"splits", "-pace", "4:30", "-dist", "10k"}, "rabbit -pace 4:30 -dist 10k"},
		{[]string{"pace", "-time", "45:30", "-dist", "10k"}, "rabbit -time 45:30 -dist 10k -only"},
	}
	for _, tt := range tests {
		err := run(tt.args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "removed in v0.4.0") || !strings.Contains(err.Error(), tt.hint) {
			t.Errorf("run(%q) error = %v, want removal message with %q", tt.args, err, tt.hint)
		}
	}
}

func TestRunOnlyRejectsTableFlags(t *testing.T) {
	for _, flag := range [][]string{{"-every", "5k"}, {"-halfway"}} {
		args := append([]string{"-pace", "4:30", "-dist", "10k", "-only"}, flag...)
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "-only") {
			t.Errorf("run(%q) error = %v, want -only conflict", args, err)
		}
	}
}

func TestRunHelpFlag(t *testing.T) {
	var buf strings.Builder
	if err := run([]string{"-pace", "4:30", "-h"}, &buf); err != nil {
		t.Errorf("-h returned error: %v", err)
	}
}
