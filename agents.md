# p

`p` is a thin, opinionated wrapper around `systemd` that makes running long-lived
processes feel like [pm2](https://pm2.keymetrics.io/). It generates real systemd
units under the hood, so you keep all of systemd's reliability (restart policies,
resource limits, log retention, boot persistence) with a friendlier CLI.

```
p start "node app.js" -n web
p status
p logs web
p restart web
p delete web
```

## Why

systemd is excellent at running services, but its UX for ad-hoc processes is
clumsy — you write a unit file, place it in the right directory, reload, enable,
start, then read four different commands to see what's happening. `p` collapses
that into one verb each, plus pretty, mobile-friendly output.

Every service `p` manages lives in a unit file named `p-<name>.service`, so you
can always drop down to plain `systemctl` if you need to.

## Install

```
go install github.com/ccev/p@latest
```

Or from source:

```
git clone https://github.com/ccev/p
cd p
go build -o p
sudo mv p /usr/local/bin/
```

Requires Go 1.25+ and a host with `systemd` and `journalctl` available.

### Modes

- Running as **root**: `p` uses the system instance — units are written to
  `/etc/systemd/system/` and everything runs under `systemctl --system`.
- Running as a **regular user**: `p` uses the user instance — units land in
  `~/.config/systemd/user/` and run under `systemctl --user`. For services to
  survive logout, enable lingering once: `sudo loginctl enable-linger $USER`.

## Commands

### `p start [command...] -n <name>`

Create a systemd unit for the given command and start it.

```
p start "node app.js" -n web
p start ./worker --serve -n worker -d /srv/worker
p start "python -u bot.py" -n bot --env TOKEN=xyz --memory-max 256M
```

Flags map directly onto common systemd unit options:

| Flag | Unit field | Notes |
|------|------------|-------|
| `-n, --name` | unit file name | required, used everywhere |
| `-D, --description` | `Description=` | shown in `status` / `show` |
| `-d, --cwd` | `WorkingDirectory=` | defaults to the current directory |
| `-u, --user` | `User=` | system mode only |
| `-g, --group` | `Group=` | system mode only |
| `-e, --env KEY=VAL` | `Environment=` | repeatable |
| `--env-file` | `EnvironmentFile=` | path to env file |
| `-r, --restart` | `Restart=` | `always` (default), `on-failure`, `no`, … |
| `--restart-sec` | `RestartSec=` | seconds to wait between restarts |
| `-m, --memory-max` | `MemoryMax=` | e.g. `256M`, `1G` |
| `-c, --cpu-quota` | `CPUQuota=` | e.g. `50%` |
| `--kill-signal` | `KillSignal=` | defaults to `SIGTERM` |
| `--timeout-stop` | `TimeoutStopSec=` | grace period before SIGKILL |
| `--start-limit-burst` | `StartLimitBurst=` | restart burst limit |
| `--umask` | `UMask=` | e.g. `0022` |
| `--after` | `After=` | ordering deps |
| `--wants` | `Wants=` | weak deps |
| `--requires` | `Requires=` | strict deps |
| `--ip-accounting` | `IPAccounting=yes` | enabled by default so `status` shows network IO |
| `--auto-start` | `enable`'s the unit | on by default; service starts at boot |
| `--no-start` | — | create the unit, don't start it yet |
| `--force` | — | replace an existing service with the same name |

### `p stop <name>`

Stops the service. The unit file stays in place so `p start` (or `p restart`)
can bring it back.

### `p restart <name>`

Restarts the service via `systemctl restart`.

### `p delete <name>`  *(aliases: `rm`, `remove`)*

Stops, disables, removes the unit file and reloads systemd. The service is gone.

### `p status [name...]`  *(aliases: `ls`, `list`, `ps`)*

Prints a compact table of every `p`-managed service:

```
id  name      status      pid     uptime    ↺   cpu    mem      net ⇣/⇡
0   web    ● online       12345   2d 3h    0   1.2%   142.0MB  4.1MB/1.2MB
1   worker ● online       12380   2d 3h    2   0.4%   58.0MB   —
2   bot    ● failed       —       —        7   —      —        —

3 service(s)  2 online  0 stopped  1 failed
```

- The terminal width is respected: low-priority columns (`id`, `pid`, `net`)
  drop out first on narrow displays (mobile / split-pane terminals).
- `-w, --watch` redraws on an interval (`--interval 2s` by default).
- `--no-cpu` skips the CPU% sample (faster, useful in scripts).

### `p logs [name...]`

Streams journald logs, colorised by log level (errors red, warnings yellow,
info cyan, debug dim). When tailing multiple services at once, each one gets a
distinct color prefix so streams stay readable.

```
p logs web
p logs web worker --lines 200
p logs --grep '5\d\d ' web
```

Flags:

| Flag | Description |
|------|-------------|
| `-l, --lines` | initial lines to print (default `50`) |
| `--no-follow` | print and exit instead of tailing |
| `--no-color` | plain output |
| `--raw` | drop the service-name prefix |
| `-s, --since` | passthrough to `journalctl --since` |
| `--grep REGEX` | filter to matching lines |

If you run `p logs` with no arguments, it tails *every* p-managed service.

### `p show <name>`

Prints all the interesting details — load state, enabled state, restart policy,
resource limits, environment, etc. — as a vertical card that reads well on
narrow terminals. `--raw` prints the actual unit file instead.

## Suggested zsh aliases

The single-letter aliases mentioned in the original spec map cleanly onto
`p`'s subcommands, but they're nicer as shell aliases than as another binary:

```sh
alias pst='p status'
alias pl='p logs'
alias pr='p restart'
```

Drop those into `~/.zshrc` and you're set.

## How it works under the hood

`p start --name web …` writes a unit like this to
`~/.config/systemd/user/p-web.service` (or `/etc/systemd/system/` as root):

```ini
# Managed by p — https://github.com/ccev/p
[Unit]
Description=p-managed service: web

[Service]
Type=simple
ExecStart=/bin/sh -c 'node app.js'
WorkingDirectory=/srv/web
Environment=NODE_ENV=production
Restart=always
RestartSec=5
MemoryMax=256M
IPAccounting=yes

[Install]
WantedBy=default.target
```

…then runs `systemctl daemon-reload`, `systemctl enable p-web`, and
`systemctl start p-web`. Stats in `p status` come from
`systemctl show -p MemoryCurrent,CPUUsageNSec,IPIngressBytes,…`. Logs come from
`journalctl -u p-web`.

Because everything is real systemd state, you can mix `p` with bare
`systemctl` and `journalctl` invocations whenever you want.

## License

MIT
