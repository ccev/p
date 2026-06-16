package systemd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const ServicePrefix = "p-"

type Mode int

const (
	ModeUser Mode = iota
	ModeSystem
)

func CurrentMode() Mode {
	if os.Geteuid() == 0 {
		return ModeSystem
	}
	return ModeUser
}

func (m Mode) Flag() string {
	if m == ModeUser {
		return "--user"
	}
	return "--system"
}

func (m Mode) UnitDir() (string, error) {
	if m == ModeSystem {
		return "/etc/systemd/system", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func systemctl(args ...string) (string, error) {
	all := append([]string{CurrentMode().Flag()}, args...)
	cmd := exec.Command("systemctl", all...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), msg)
	}
	return out.String(), nil
}

func DaemonReload() error {
	_, err := systemctl("daemon-reload")
	return err
}

func ResetFailed(name string) error {
	_, err := systemctl("reset-failed", ServiceUnit(name))
	return err
}

func Start(name string) error {
	_, err := systemctl("start", "--no-block", ServiceUnit(name))
	return err
}

func Stop(name string) error {
	_, err := systemctl("stop", "--no-block", ServiceUnit(name))
	return err
}

func Restart(name string) error {
	_, err := systemctl("restart", "--no-block", ServiceUnit(name))
	return err
}

func Enable(name string) error {
	_, err := systemctl("enable", ServiceUnit(name))
	return err
}

func Disable(name string) error {
	_, err := systemctl("disable", ServiceUnit(name))
	return err
}

func Kill(name, signal, whom string) error {
	args := []string{"kill"}
	if signal != "" {
		args = append(args, "-s", signal)
	}
	if whom != "" {
		args = append(args, "--kill-whom="+whom)
	}
	args = append(args, ServiceUnit(name))
	_, err := systemctl(args...)
	return err
}

func ServiceUnit(name string) string {
	if strings.HasSuffix(name, ".service") {
		return name
	}
	return ServicePrefix + name + ".service"
}

func StripPrefix(unit string) string {
	n := strings.TrimSuffix(unit, ".service")
	return strings.TrimPrefix(n, ServicePrefix)
}

func Show(name string, props ...string) (map[string]string, error) {
	args := []string{"show", ServiceUnit(name)}
	if len(props) > 0 {
		args = append(args, "-p", strings.Join(props, ","))
	}
	out, err := systemctl(args...)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	s := bufio.NewScanner(strings.NewReader(out))
	for s.Scan() {
		line := s.Text()
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		result[line[:idx]] = line[idx+1:]
	}
	return result, nil
}

func List() ([]string, error) {
	dir, err := CurrentMode().UnitDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, ServicePrefix) && strings.HasSuffix(n, ".service") {
			names = append(names, StripPrefix(n))
		}
	}
	return names, nil
}

func Exists(name string) bool {
	dir, err := CurrentMode().UnitDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, ServiceUnit(name)))
	return err == nil
}
