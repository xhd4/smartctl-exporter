## Features

**smartctl-exporter** — Windows service with in-process [prometheus-community/smartctl_exporter](https://github.com/prometheus-community/smartctl_exporter) **v0.14.0**.

- Single binary: `smartctl-exporter.exe` (no separate child exporter)
- windows_exporter-compatible `config.yaml`, `--log.level`, and env `LOG_LEVEL`
- `--install` / `--uninstall` / `--dry-run` (Program Files, autostart service, firewall)
- Auto-install smartmontools via winget, with GitHub Releases fallback

## Downloads

Download `smartctl-exporter-windows-amd64.zip`, extract, then run elevated:

```text
smartctl-exporter.exe --install
```
