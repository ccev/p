package systemd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestKillCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		help       string
		signal     string
		whom       string
		helpError  string
		killError  string
		wantOption string
		wantError  string
	}{
		{name: "legacy systemctl", help: "--kill-who=WHO", signal: "SIGHUP", whom: "main", wantOption: "--kill-who=main"},
		{name: "modern systemctl", help: "--kill-whom=WHOM", signal: "SIGHUP", whom: "main", wantOption: "--kill-whom=main"},
		{name: "prefer current spelling", help: "--kill-who=WHO --kill-whom=WHOM", signal: "SIGUSR1", whom: "all", wantOption: "--kill-whom=all"},
		{name: "default signal", help: "--kill-who=WHO", whom: "main", wantOption: "--kill-who=main"},
		{name: "default recipients", signal: "SIGUSR2"},
		{name: "help failure", whom: "main", helpError: "help unavailable", wantError: "detect systemctl kill options:"},
		{name: "unsupported selector", help: "--signal=SIGNAL", whom: "main", wantError: "systemctl does not support selecting kill recipients"},
		{name: "kill failure is not retried", help: "--kill-whom=WHOM", signal: "SIGHUP", whom: "main", killError: "Access denied", wantOption: "--kill-whom=main", wantError: "Access denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$P_TEST_CALLS"
case "$2" in
  --help)
    if [ -n "$P_TEST_HELP_ERROR" ]; then
      printf '%s\n' "$P_TEST_HELP_ERROR" >&2
      exit 1
    fi
    printf '%s\n' "$P_TEST_HELP"
    ;;
  kill)
    if [ -n "$P_TEST_KILL_ERROR" ]; then
      printf '%s\n' "$P_TEST_KILL_ERROR" >&2
      exit 1
    fi
    ;;
  *) exit 2 ;;
esac
`
			if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			// No real systemctl or service is invoked by these tests.
			t.Setenv("PATH", dir)
			t.Setenv("P_TEST_CALLS", calls)
			t.Setenv("P_TEST_HELP", tt.help)
			t.Setenv("P_TEST_HELP_ERROR", tt.helpError)
			t.Setenv("P_TEST_KILL_ERROR", tt.killError)

			err := Kill("example", tt.signal, tt.whom)
			if tt.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}

			mode := CurrentMode().Flag()
			var want []string
			if tt.whom != "" {
				want = append(want, mode+" --help")
			}
			if tt.wantError == "" || tt.killError != "" {
				args := []string{mode, "kill"}
				if tt.signal != "" {
					args = append(args, "-s", tt.signal)
				}
				if tt.wantOption != "" {
					args = append(args, tt.wantOption)
				}
				want = append(want, strings.Join(append(args, "p-example.service"), " "))
			}
			data, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Split(strings.TrimSpace(string(data)), "\n")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("systemctl calls = %q, want %q", got, want)
			}
		})
	}
}
