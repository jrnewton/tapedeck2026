package main

import (
	"testing"
)

func TestScheduleDownload_MissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"only station", []string{"WMBR"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdScheduleDownload(tc.args)
			if err == nil {
				t.Error("expected error for missing arguments")
			}
		})
	}
}
