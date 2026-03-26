package api

import (
	"testing"
)

func TestSplitVolumeUnix(t *testing.T) {
	tests := []struct {
		input  string
		source string
		target string
	}{
		{"/host/path:/container/path", "/host/path", "/container/path"},
		{"/data:/var/lib/data", "/data", "/var/lib/data"},
		{"myvolume", "myvolume", ""},
		{"", "", ""},
	}

	for _, tc := range tests {
		parts := splitVolume(tc.input)
		source := parts[0]
		target := ""
		if len(parts) > 1 {
			target = parts[1]
		}
		if source != tc.source {
			t.Errorf("splitVolume(%q) source = %q, want %q", tc.input, source, tc.source)
		}
		if target != tc.target {
			t.Errorf("splitVolume(%q) target = %q, want %q", tc.input, target, tc.target)
		}
	}
}

func TestSplitVolumeWindows(t *testing.T) {
	tests := []struct {
		input  string
		source string
		target string
	}{
		{`D:\data\postgres:/var/lib/postgresql/data`, `D:\data\postgres`, "/var/lib/postgresql/data"},
		{`C:\Users\app:/app`, `C:\Users\app`, "/app"},
		{`D:\data`, `D:\data`, ""},
	}

	for _, tc := range tests {
		parts := splitVolume(tc.input)
		source := parts[0]
		target := ""
		if len(parts) > 1 {
			target = parts[1]
		}
		if source != tc.source {
			t.Errorf("splitVolume(%q) source = %q, want %q", tc.input, source, tc.source)
		}
		if target != tc.target {
			t.Errorf("splitVolume(%q) target = %q, want %q", tc.input, target, tc.target)
		}
	}
}

func TestSplitVolumeEmpty(t *testing.T) {
	parts := splitVolume("")
	if len(parts) != 1 || parts[0] != "" {
		t.Errorf("splitVolume(\"\") = %v, want [\"\"]", parts)
	}
}
