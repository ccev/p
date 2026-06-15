package systemd

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// ParseUnit reads the on-disk unit for name and returns a UnitConfig
// populated from the keys p knows about. Unknown keys are silently
// dropped; if you need round-trip preservation, use Show() instead.
func ParseUnit(name string) (*UnitConfig, error) {
	path, err := UnitPath(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &UnitConfig{Name: name}
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		raw := sc.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		switch section {
		case "Unit":
			switch key {
			case "Description":
				cfg.Description = val
			case "After":
				cfg.After = strings.Fields(val)
			case "Wants":
				cfg.Wants = strings.Fields(val)
			case "Requires":
				cfg.Requires = strings.Fields(val)
			case "StartLimitBurst":
				cfg.StartLimitBurst, _ = strconv.Atoi(val)
			}
		case "Service":
			switch key {
			case "ExecStart":
				cfg.Command = parseExecStart(val)
			case "WorkingDirectory":
				cfg.WorkingDir = val
			case "User":
				cfg.User = val
			case "Group":
				cfg.Group = val
			case "Environment":
				cfg.Env = append(cfg.Env, val)
			case "EnvironmentFile":
				cfg.EnvFile = val
			case "Restart":
				cfg.Restart = val
			case "RestartSec":
				cfg.RestartSec, _ = strconv.Atoi(val)
			case "MemoryMax":
				cfg.MemoryMax = val
			case "CPUQuota":
				cfg.CPUQuota = val
			case "StandardOutput":
				cfg.StandardOutput = val
			case "StandardError":
				cfg.StandardError = val
			case "KillSignal":
				cfg.KillSignal = val
			case "TimeoutStopSec":
				cfg.TimeoutStopSec, _ = strconv.Atoi(val)
			case "UMask":
				cfg.UMask = val
			case "IPAccounting":
				cfg.IPAccounting = strings.EqualFold(val, "yes") || strings.EqualFold(val, "true")
			}
		}
	}
	return cfg, sc.Err()
}

// parseExecStart strips systemd's ExecStart prefixes (@, -, +, !, !!, :),
// then unwraps our `/bin/sh -c '...'` wrapper if present.
func parseExecStart(s string) string {
	s = strings.TrimSpace(s)
	for len(s) > 0 {
		c := s[0]
		if c == '@' || c == '-' || c == '+' || c == '!' || c == ':' {
			s = s[1:]
			continue
		}
		break
	}
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "/bin/sh -c "); ok {
		return shellUnquote(strings.TrimSpace(rest))
	}
	return s
}

func shellUnquote(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], `'\''`, "'")
	}
	return s
}
