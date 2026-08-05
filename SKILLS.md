# SKILLS.md — smartctl-exporter contributor guide

Guide for humans and coding agents working on this repository.

## Purpose

Windows service wrapping [prometheus-community/smartctl_exporter](https://github.com/prometheus-community/smartctl_exporter) **in-process** (vendored under `internal/exporter`). Not a separate child `.exe`.

## Architecture

```text
cmd/smartctl-exporter/main.go  → CLI (help, install, run service/foreground)
internal/exporter                  → vendored upstream v0.14.0 (exporter.Run)
internal/config                    → YAML + CLI + env merge (windows_exporter-style)
internal/install                   → --install / --uninstall (sc, copy, Program Files)
internal/firewall                  → New-NetFirewallRule / delete by prefix
internal/smartmontools             → winget, then GitHub Releases setup.exe
```

Config priority: **CLI > config.yaml > env > defaults**.

Firewall rule points at the **same** `smartctl-exporter.exe` that listens for metrics.

## Key paths

| Path | Role |
|------|------|
| `cmd/smartctl-exporter/` | Windows host entrypoint |
| `internal/exporter/` | Vendored upstream collector + HTTP |
| `internal/` | config, install, firewall, smartmontools |
| `dist/windows-amd64/` | Local Docker build output |
| `dist/smartctl-exporter-amd64.exe` | Release binary (amd64) |
| `dist/smartctl-exporter-arm64.exe` | Release binary (arm64) |
| `config.yaml.example` | Committed template |
| `config.yaml` | Local overrides (gitignored) |
| `%ProgramFiles%\smartctl-exporter\` | Production install |
| `%ProgramData%\smartctl-exporter\logs\` | Default logs |
| `scripts/start-service.ps1` / `stop-service.ps1` | Make docker-win only |

## Build

```bash
make package-win
# or
make docker-build-host
```

Version is LDFLAGS `-X main.version=` from `SMARTCTL_EXPORTER_VERSION` (default `v0.14.0`).

## Adding a config key

1. Add field + default in `internal/config`
2. Wire flat key (`web.listen-address`) and YAML serialize
3. Add CLI parsing in `main.go` / env map
4. Map into `ChildArgs()` if the exporter needs it
5. Update `config.yaml.example` and `README.md`

## Do not

- Reintroduce a separate `smartctl_exporter.exe` child
- Reintroduce `service.bat` or `.env`-only install flow
- Commit `config.yaml`, `.env`, `dist/`, or `logs/`
- Bundle smartmontools inside the release (install at `--install` time)

## Release

Tag `v*` → GitHub Actions builds `smartctl-exporter-amd64.exe` and `smartctl-exporter-arm64.exe`.
