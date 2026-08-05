## Features

**smartctl-exporter** — Windows service with in-process [prometheus-community/smartctl_exporter](https://github.com/prometheus-community/smartctl_exporter) **v0.14.0**.

- Single binary (in-process exporter)
- Release assets: `smartctl-exporter-amd64.exe`, `smartctl-exporter-arm64.exe`
- windows_exporter-compatible `config.yaml`, `--log.level`, and env `LOG_LEVEL`
- `--install` / `--uninstall` / `--dry-run` (Program Files, autostart service, firewall)
- Auto-install smartmontools via winget, with GitHub Releases fallback

## Downloads

Download the matching `.exe`, then run elevated:

```text
smartctl-exporter-amd64.exe --install
```
