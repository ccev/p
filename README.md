# p

A thin, friendly wrapper around `systemd`. Run long-lived processes with a CLI
that feels like [pm2](https://pm2.keymetrics.io/), but every service is a real
systemd unit you can poke at with `systemctl` whenever you want.

Run `p <command> --help` for the full flag list of any subcommand.

## Install

```sh
go install github.com/ccev/p@latest
```

Requires Go 1.25+, `systemd`, and `journalctl`. Run as your user (units land in
`~/.config/systemd/user/`) or as root (system-wide units).

## Update

Same command — `go install` always fetches the latest tagged version and
overwrites the binary in `$GOBIN` (usually `~/go/bin/p`):

```sh
go install github.com/ccev/p@latest
```

Existing services keep running on the old binary's unit format. To re-render
one through the current template (e.g. to pick up the login-shell wrapping),
do any declarative edit:

```sh
p edit <name> --cmd "<same or new command>"
```

## Examples

### Start something

```sh
$ p start "node app.js" -n web
● web
  unit   /home/me/.config/systemd/user/p-web.service
  cmd    node app.js
  cwd    /srv/web
  state  started
```

### See what's running

```sh
$ p status
id  name    status      pid     uptime   ↺   cpu    mem      net ⇣/⇡
0   web   ● online      12345   2d 3h    0   1.2%   142.0MB  4.1MB/1.2MB
1   worker● online      12380   2d 3h    2   0.4%   58.0MB   —
2   bot   ● failed      —       —        7   —      —        —

3 service(s)  2 online  0 stopped  1 failed
```

`p status -w` keeps it refreshing. Columns drop out on narrow terminals so it
still reads on a phone.

### Tail the logs

```sh
$ p logs web
2026-06-15T09:42:01+0000 p-web.service[12345]: listening on :3000
2026-06-15T09:42:09+0000 p-web.service[12345]: GET / 200 12ms
2026-06-15T09:42:11+0000 p-web.service[12345]: warning slow query (134ms)
```

`-l 200` for more scrollback, `--grep` to filter, pass multiple names to
multiplex with per-service color prefixes.

### Restart, stop, reload, delete

```sh
$ p restart web
● web restarted

$ p reload web                # SIGHUP — re-read config without restarting
● web reloaded (SIGHUP → pid 12345)

$ p stop web
● web stopped

$ p delete web
✖ web deleted
```

### Logs are size-capped per service

Each service's journal is isolated and capped at 20MB by default — older
entries get rotated out automatically, no cron needed. Override per-service
with `--log-max` (e.g. `--log-max 100M`, `--log-max 1G`), and clear a
service's logs on demand with `p flush`:

```sh
$ p flush web
● web logs flushed

$ p flush --all
```

### Inspect or edit a service

```sh
$ p show web
● web  online
  description  p-managed service: web
  unit         /home/me/.config/systemd/user/p-web.service
  pid          12345
  uptime       2h13m
  memory       142.0MB
  cpu          1.2%
  command      /bin/sh -c 'node app.js'
  cwd          /srv/web
  restart      always (after 5s)

$ p edit web --memory-max 512M --env DEBUG=1
● web edited & restarted
```

`p edit web` with no flags opens the unit in `$EDITOR`.

### Suggested zsh aliases

```sh
alias pst='p status'
alias pl='p logs'
alias pr='p restart'
```

---

*This repository was written entirely by an AI agent.*
