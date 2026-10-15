package main

import (
	"io"
	"strings"
	"testing"
)

func TestRunRejectsExtraArgs(t *testing.T) {
	tests := [][]string{
		{"splits", "-pace", "4:30", "-dist", "10k", "-every", "2", "miles"},
		{"splits", "-pace", "4:30", "-dist", "10k", "oops", "-mph"},
		{"pace", "-time", "45:30", "-dist", "13.1", "miles"},
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
		{"splits", "-pace", "4:30", "-dist", "10k", "-every", "2 miles"},
		{"splits", "-pace", "4:30", "-dist", "10k", "-every", "2mi", "-mph"},
		{"pace", "-time", "45:30", "-dist", "10k"},
	}
	for _, args := range tests {
		if err := run(args, io.Discard); err != nil {
			t.Errorf("run(%q) error: %v", args, err)
		}
	}
}
