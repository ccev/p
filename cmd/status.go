package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var (
	statusWatch    bool
	statusInterval time.Duration
	statusNoCPU    bool
)

var statusCmd = &cobra.Command{
	Use:     "status [name...]",
	Aliases: []string{"ls", "list", "ps"},
	Short:   "Show overview of all p-managed services",
	RunE: func(cmd *cobra.Command, args []string) error {
		if statusWatch {
			return runStatusWatch(args)
		}
		return renderStatus(args, os.Stdout)
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "refresh continuously")
	statusCmd.Flags().DurationVar(&statusInterval, "interval", 2*time.Second, "refresh interval when --watch is set")
	statusCmd.Flags().BoolVar(&statusNoCPU, "no-cpu", false, "skip CPU% sampling (faster)")
}

func runStatusWatch(args []string) error {
	for {
		fmt.Print("\x1b[2J\x1b[H")
		if err := renderStatus(args, os.Stdout); err != nil {
			return err
		}
		fmt.Printf("\n%s\n", ui.Dim.Sprintf("refresh every %s — ctrl+c to exit", statusInterval))
		time.Sleep(statusInterval)
	}
}

func renderStatus(filter []string, w *os.File) error {
	names, err := systemd.List()
	if err != nil {
		return err
	}
	if len(filter) > 0 {
		set := make(map[string]bool)
		for _, n := range filter {
			set[n] = true
		}
		filtered := names[:0]
		for _, n := range names {
			if set[n] {
				filtered = append(filtered, n)
			}
		}
		names = filtered
	}
	if len(names) == 0 {
		fmt.Fprintln(w, ui.Dim.Sprint("no services. use `p start <command> -n <name>` to create one."))
		return nil
	}
	sort.Strings(names)

	type row struct {
		id    int
		name  string
		stats *systemd.Stats
		cpu   float64
	}
	rows := make([]row, len(names))
	var wg sync.WaitGroup
	for i, n := range names {
		i, n := i, n
		rows[i] = row{id: i, name: n}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := systemd.GetStats(n)
			if err != nil {
				return
			}
			rows[i].stats = s
			if !statusNoCPU && s.IsRunning() {
				if c, err := systemd.SampleCPUPercent(n, 250*time.Millisecond); err == nil {
					rows[i].cpu = c
				}
			}
		}()
	}
	wg.Wait()

	tbl := ui.Table{
		Columns: []ui.Column{
			{Header: "id", Align: ui.AlignRight, Priority: 2},
			{Header: "name", Priority: 0, MaxWidth: 24},
			{Header: "", Priority: 0}, // status dot
			{Header: "status", Priority: 0},
			{Header: "pid", Align: ui.AlignRight, Priority: 3},
			{Header: "uptime", Align: ui.AlignRight, Priority: 1},
			{Header: "↺", Align: ui.AlignRight, Priority: 2},
			{Header: "cpu", Align: ui.AlignRight, Priority: 1},
			{Header: "mem", Align: ui.AlignRight, Priority: 1},
			{Header: "net ⇣/⇡", Align: ui.AlignRight, Priority: 3},
		},
	}
	online, stopped, failed := 0, 0, 0
	for _, r := range rows {
		s := r.stats
		var pid, uptime, cpu, mem, net, restarts, statusLbl, dot string
		if s == nil {
			statusLbl = ui.Dim.Sprint("unknown")
			dot = ui.Dim.Sprint("●")
			pid, uptime, cpu, mem, net, restarts = "—", "—", "—", "—", "—", "—"
		} else {
			dot = ui.StatusDot(s.Active, s.Sub)
			statusLbl = ui.StatusLabel(s.Active, s.Sub)
			if s.PID > 0 {
				pid = strconv.Itoa(s.PID)
			} else {
				pid = ui.Dim.Sprint("—")
			}
			uptime = ui.Duration(s.Uptime())
			if s.IsRunning() {
				cpu = ui.Percent(r.cpu)
			} else {
				cpu = ui.Dim.Sprint("—")
			}
			mem = ui.Bytes(s.MemoryBytes())
			if s.IPIngress == 0 && s.IPEgress == 0 {
				net = ui.Dim.Sprint("—")
			} else {
				net = ui.Bytes(s.IPIngress) + "/" + ui.Bytes(s.IPEgress)
			}
			restarts = strconv.FormatUint(uint64(s.Restarts), 10)
			switch s.Active {
			case "active":
				online++
			case "failed":
				failed++
			default:
				stopped++
			}
		}
		tbl.Rows = append(tbl.Rows, []string{
			ui.Dim.Sprint(strconv.Itoa(r.id)),
			ui.Bold.Sprint(r.name),
			dot,
			statusLbl,
			pid,
			uptime,
			restarts,
			cpu,
			mem,
			net,
		})
	}
	tbl.Render(w)
	fmt.Fprintf(w, "\n%s  %s  %s  %s\n",
		ui.Label.Sprintf("%d service(s)", len(names)),
		ui.Green.Sprintf("%d online", online),
		ui.Yellow.Sprintf("%d stopped", stopped),
		ui.Red.Sprintf("%d failed", failed),
	)
	return nil
}
