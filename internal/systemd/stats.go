package systemd

import (
	"strconv"
	"time"
)

const memoryInvalid = ^uint64(0)

type Stats struct {
	Name          string
	Active        string
	Sub           string
	PID           int
	Memory        uint64
	CPUUsageNSec  uint64
	IPIngress     uint64
	IPEgress      uint64
	StartedAt     time.Time
	Restarts      uint
	UnitFileState string
	LoadState     string
	Result        string
	Description   string
	ExecMainCode  string
	ExecMainStatus string
}

func GetStats(name string) (*Stats, error) {
	props := []string{
		"ActiveState", "SubState", "MainPID", "MemoryCurrent",
		"CPUUsageNSec", "IPIngressBytes", "IPEgressBytes",
		"ExecMainStartTimestamp", "NRestarts", "UnitFileState",
		"LoadState", "Result", "Description",
		"ExecMainCode", "ExecMainStatus",
	}
	m, err := Show(name, props...)
	if err != nil {
		return nil, err
	}
	s := &Stats{Name: name}
	s.Active = m["ActiveState"]
	s.Sub = m["SubState"]
	s.PID, _ = strconv.Atoi(m["MainPID"])
	s.Memory = parseU64(m["MemoryCurrent"])
	s.CPUUsageNSec = parseU64(m["CPUUsageNSec"])
	s.IPIngress = parseU64(m["IPIngressBytes"])
	s.IPEgress = parseU64(m["IPEgressBytes"])
	s.Restarts = uint(parseU64(m["NRestarts"]))
	s.UnitFileState = m["UnitFileState"]
	s.LoadState = m["LoadState"]
	s.Result = m["Result"]
	s.Description = m["Description"]
	s.ExecMainCode = m["ExecMainCode"]
	s.ExecMainStatus = m["ExecMainStatus"]
	if ts := m["ExecMainStartTimestamp"]; ts != "" {
		if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", ts); err == nil {
			s.StartedAt = t
		}
	}
	return s, nil
}

func parseU64(s string) uint64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func SampleCPUPercent(name string, window time.Duration) (float64, error) {
	a, err := GetStats(name)
	if err != nil {
		return 0, err
	}
	if !a.IsRunning() {
		return 0, nil
	}
	time.Sleep(window)
	b, err := GetStats(name)
	if err != nil {
		return 0, err
	}
	if b.CPUUsageNSec < a.CPUUsageNSec {
		return 0, nil
	}
	delta := b.CPUUsageNSec - a.CPUUsageNSec
	return 100 * float64(delta) / float64(window.Nanoseconds()), nil
}

func (s *Stats) MemoryBytes() uint64 {
	if s.Memory == memoryInvalid {
		return 0
	}
	return s.Memory
}

func (s *Stats) IsRunning() bool {
	return s.Active == "active" && s.Sub == "running"
}

func (s *Stats) Uptime() time.Duration {
	if s.StartedAt.IsZero() || !s.IsRunning() {
		return 0
	}
	return time.Since(s.StartedAt)
}
