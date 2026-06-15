package systemd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultLogMaxSize is the per-service journal cap applied when the user
// hasn't given an explicit --log-max value. Matches the size requested
// when the feature was first introduced.
const DefaultLogMaxSize = "20M"

// Namespace is the LogNamespace value used for service name. Each managed
// service gets its own journald instance so its logs can be size-capped
// independently of the global journal.
func Namespace(name string) string {
	return "p-" + name
}

// JournaldConfExists reports whether a per-namespace journald drop-in
// has already been written. Used to detect old units that have a
// LogNamespace= directive but no size cap yet.
func JournaldConfExists(namespace string) bool {
	dir, err := journaldConfDir(namespace)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "p.conf"))
	return err == nil
}

// journaldConfDir returns the directory where we drop per-namespace
// journald config. For system mode this is /etc/systemd/journald@<ns>.conf.d/.
// User mode mirrors that in ~/.config/systemd/.
func journaldConfDir(namespace string) (string, error) {
	if CurrentMode() == ModeSystem {
		return "/etc/systemd/journald@" + namespace + ".conf.d", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config/systemd/journald@"+namespace+".conf.d"), nil
}

// WriteJournaldConf creates the journald drop-in that caps the namespace's
// total log size to maxSize. Sets both SystemMaxUse (persistent storage)
// and RuntimeMaxUse (volatile) so the cap holds regardless of which
// storage mode journald ends up using.
func WriteJournaldConf(namespace, maxSize string) error {
	if maxSize == "" {
		return nil
	}
	dir, err := journaldConfDir(namespace)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	body := fmt.Sprintf(`# Managed by p — https://github.com/ccev/p
[Journal]
SystemMaxUse=%s
RuntimeMaxUse=%s
`, maxSize, maxSize)
	return os.WriteFile(filepath.Join(dir, "p.conf"), []byte(body), 0644)
}

// DeleteJournaldConf removes the per-namespace journald config dir.
func DeleteJournaldConf(namespace string) error {
	dir, err := journaldConfDir(namespace)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// DeleteJournalFiles removes the namespace's journal directories from
// both persistent and volatile storage. Best-effort: missing dirs are
// not an error.
func DeleteJournalFiles(namespace string) error {
	var paths []string
	if CurrentMode() == ModeSystem {
		paths = []string{
			"/var/log/journal." + namespace,
			"/run/log/journal." + namespace,
		}
	} else {
		if state := os.Getenv("XDG_STATE_HOME"); state != "" {
			paths = append(paths, filepath.Join(state, "log/journal."+namespace))
		} else if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(home, ".local/state/log/journal."+namespace))
		}
	}
	for _, p := range paths {
		_ = os.RemoveAll(p)
	}
	return nil
}

// RestartJournald asks systemd to restart the namespace's journald
// instance — needed after WriteJournaldConf so the new SystemMaxUse takes
// effect. No-op when the instance isn't running yet (try-restart).
func RestartJournald(namespace string) error {
	_, err := systemctl("try-restart", "systemd-journald@"+namespace+".service")
	return err
}

// StopJournald stops the namespace's journald instance — used by `p delete`
// before the conf is wiped so the next activation starts fresh.
func StopJournald(namespace string) error {
	_, err := systemctl("stop", "systemd-journald@"+namespace+".service")
	return err
}

// FlushJournal deletes all journal entries in the namespace by rotating
// the active file and vacuuming everything older than 1 second.
//
// When the namespace's journald instance has never been activated (no log
// has been written), journalctl errors with "No such file or directory" —
// we treat that as success because the namespace is already empty.
func FlushJournal(namespace string) error {
	if _, err := journalctl(namespace, "--rotate"); err != nil && !isNamespaceMissing(err) {
		return err
	}
	if _, err := journalctl(namespace, "--vacuum-time=1s"); err != nil && !isNamespaceMissing(err) {
		return err
	}
	return nil
}

func isNamespaceMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "No such file or directory")
}

func journalctl(namespace string, args ...string) (string, error) {
	all := []string{CurrentMode().Flag()}
	if namespace != "" {
		all = append(all, "--namespace="+namespace)
	}
	all = append(all, args...)
	cmd := exec.Command("journalctl", all...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("journalctl %s: %s", strings.Join(args, " "), msg)
	}
	return out.String(), nil
}
