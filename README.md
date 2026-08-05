# smartctl-exporter

Windows service that exposes [prometheus-community/smartctl_exporter](https://github.com/prometheus-community/smartctl_exporter) S.M.A.R.T. metrics.

One binary: `smartctl-exporter.exe` installs as a Windows service, manages firewall rules, can auto-install [smartmontools](https://github.com/smartmontools/smartmontools), and runs the exporter **in-process** (vendored upstream v0.14.0).

Configuration follows windows_exporter-style conventions: `config.yaml`, `--log.level`, and env `LOG_LEVEL`.

## Requirements

- Windows 10/11 **amd64** or **arm64**
- Administrator privileges for `--install` / `--uninstall`
- [Docker](https://www.docker.com/) to cross-compile (or Go with `GOOS=windows`)
- smartmontools (`smartctl`) — installed automatically on `--install` if missing (winget, then GitHub Releases fallback)

## Build

```bash
make package-win              # dist/smartctl-exporter-amd64.exe
make package-win GOARCH=arm64 # dist/smartctl-exporter-arm64.exe
```

```bash
make docker-build-host
```

## Install

Download the matching release `.exe`, then run **elevated**:

```powershell
.\smartctl-exporter-amd64.exe --install
```

Installs to `%ProgramFiles%\smartctl-exporter\` as `smartctl-exporter.exe`, creates `./config.yaml` in the current directory if missing (then copies it into Program Files on first install), registers service `smartctl-exporter`, ensures firewall, and installs smartmontools if needed.

Preview:

```powershell
.\smartctl-exporter.exe --install --dry-run
```

Custom settings (written into new `config.yaml` only):

```powershell
.\smartctl-exporter.exe --install --web.listen-address=:9633 --telemetry.path=/secret/metrics
```

## Uninstall

```powershell
.\smartctl-exporter.exe --uninstall
```

Removes the service (autostart), firewall rules, `%ProgramFiles%\smartctl-exporter\`, and `%ProgramData%\smartctl-exporter\`.

## Configuration

Priority: **CLI > config.yaml > env > defaults**.

```powershell
copy config.yaml.example config.yaml
.\smartctl-exporter.exe --config.file=config.yaml --print-config
```

### config.yaml example

```yaml
log:
  level: warn
  file: C:\ProgramData\smartctl-exporter\logs\exporter.log

telemetry:
  path: /metrics

web:
  listen-address: :9633

smartctl:
  path: C:\Program Files\smartmontools\bin\smartctl.exe
  installer_version: "7.5"

firewall:
  enabled: true
  profile: any

debug:
  enabled: false
```

### Flags / env

| CLI | Env | Default |
|-----|-----|---------|
| `--web.listen-address` | `LISTEN_ADDR` | `:9633` |
| `--telemetry.path` | `TELEMETRY_PATH` | `/metrics` |
| `--log.level` | `LOG_LEVEL` | `warn` |
| `--log.file` | `LOG_FILE` | ProgramData log path |
| `--smartctl.path` | `SMARTCTL_PATH` | Program Files smartctl |
| `--firewall.enabled` | `FW_ENABLED` | `true` |
| `--firewall.profile` | `FW_PROFILE` | `any` |

Legacy env `URL_SECURE_PATH_NODE_SMARTCTL_EXPORTER` maps to `telemetry.path` (token → `/token/metrics`).

## Prometheus example

```yaml
scrape_configs:
  - job_name: smartctl-exporter
    static_configs:
      - targets: ["windows-host:9633"]
    metrics_path: /metrics
```

## License

- Host / Windows service code: MIT — see [LICENSE](LICENSE).
- Vendored [smartctl_exporter](https://github.com/prometheus-community/smartctl_exporter) (v0.14.0): Apache-2.0 — see [LICENSE.Apache-2.0](LICENSE.Apache-2.0).
- smartmontools: its own license.
