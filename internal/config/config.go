package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ExeDirectory   string
	ConfigFilePath string

	WebListenAddress string
	TelemetryPath    string
	LogFile          string
	LogLevel         string
	SmartctlPath     string
	InstallerVersion string
	FirewallEnabled  bool
	FirewallProfile  string
	DebugEnabled     bool
}

func Defaults(exeDir, configPath string) Config {
	if configPath == "" {
		configPath = filepath.Join(exeDir, "config.yaml")
	}
	return Config{
		ExeDirectory:     exeDir,
		ConfigFilePath:   configPath,
		WebListenAddress: ":9633",
		TelemetryPath:    "/metrics",
		LogFile:          `C:\ProgramData\smartctl-exporter\logs\exporter.log`,
		LogLevel:         "warn",
		SmartctlPath:     `C:\Program Files\smartmontools\bin\smartctl.exe`,
		InstallerVersion: "7.5",
		FirewallEnabled:  true,
		FirewallProfile:  "any",
		DebugEnabled:     false,
	}
}

func Load(exeDir string, configFile string, cliOverrides map[string]string) (Config, error) {
	cfg := Defaults(exeDir, configFile)
	flat := cfg.toFlat()

	mergeEnv(flat)
	if configFile != "" {
		if b, err := os.ReadFile(configFile); err == nil {
			yflat, err := flattenYAML(b)
			if err != nil {
				return Config{}, err
			}
			for k, v := range yflat {
				flat[normalizeKey(k)] = v
			}
		} else if !os.IsNotExist(err) {
			return Config{}, err
		}
	}
	for k, v := range cliOverrides {
		if v != "" {
			flat[normalizeKey(k)] = v
		}
	}

	return fromFlat(exeDir, configFile, flat), nil
}

func (c Config) ListenPort() (int, error) {
	addr := strings.TrimSpace(c.WebListenAddress)
	if strings.HasPrefix(addr, ":") {
		return strconv.Atoi(addr[1:])
	}
	if i := strings.LastIndex(addr, ":"); i >= 0 && i < len(addr)-1 {
		return strconv.Atoi(addr[i+1:])
	}
	return 0, fmt.Errorf("invalid web.listen-address: %s", c.WebListenAddress)
}

func (c Config) ChildArgs() []string {
	listen := c.WebListenAddress
	if strings.HasPrefix(listen, ":") {
		listen = "0.0.0.0" + listen
	}
	return []string{
		"--web.listen-address", listen,
		"--web.telemetry-path", c.TelemetryPath,
		"--smartctl.path", c.SmartctlPath,
		"--log.level", c.LogLevel,
	}
}

func (c Config) LogToFile() bool {
	f := strings.ToLower(strings.TrimSpace(c.LogFile))
	return f != "" && f != "stdout" && f != "stderr"
}

func (c Config) ResolvedLogFile() string {
	if filepath.IsAbs(c.LogFile) {
		return c.LogFile
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(c.LogFile, `.\\`), `./`)
	return filepath.Join(c.ExeDirectory, filepath.FromSlash(rel))
}

func (c Config) SerializeYAML() (string, error) {
	port, _ := c.ListenPort()
	_ = port
	root := map[string]any{
		"log": map[string]any{
			"level": c.LogLevel,
			"file":  c.LogFile,
		},
		"telemetry": map[string]any{
			"path": c.TelemetryPath,
		},
		"web": map[string]any{
			"listen-address": c.WebListenAddress,
		},
		"smartctl": map[string]any{
			"path":              c.SmartctlPath,
			"installer_version": c.InstallerVersion,
		},
		"firewall": map[string]any{
			"enabled": c.FirewallEnabled,
			"profile": c.FirewallProfile,
		},
		"debug": map[string]any{
			"enabled": c.DebugEnabled,
		},
	}
	b, err := yaml.Marshal(root)
	return string(b), err
}

func (c Config) toFlat() map[string]string {
	return map[string]string{
		"web.listen-address":         c.WebListenAddress,
		"telemetry.path":             c.TelemetryPath,
		"log.file":                   c.LogFile,
		"log.level":                  c.LogLevel,
		"smartctl.path":              c.SmartctlPath,
		"smartctl.installer-version": c.InstallerVersion,
		"firewall.enabled":           strconv.FormatBool(c.FirewallEnabled),
		"firewall.profile":           c.FirewallProfile,
		"debug.enabled":              strconv.FormatBool(c.DebugEnabled),
	}
}

func fromFlat(exeDir, configPath string, flat map[string]string) Config {
	d := Defaults(exeDir, configPath)
	return Config{
		ExeDirectory:     exeDir,
		ConfigFilePath:   configPath,
		WebListenAddress: get(flat, "web.listen-address", d.WebListenAddress),
		TelemetryPath:    NormalizeTelemetryPath(get(flat, "telemetry.path", d.TelemetryPath)),
		LogFile:          get(flat, "log.file", d.LogFile),
		LogLevel:         get(flat, "log.level", d.LogLevel),
		SmartctlPath:     get(flat, "smartctl.path", d.SmartctlPath),
		InstallerVersion: get(flat, "smartctl.installer-version", d.InstallerVersion),
		FirewallEnabled:  getBool(flat, "firewall.enabled", d.FirewallEnabled),
		FirewallProfile:  normalizeProfile(get(flat, "firewall.profile", d.FirewallProfile)),
		DebugEnabled:     getBool(flat, "debug.enabled", d.DebugEnabled),
	}
}

func mergeEnv(flat map[string]string) {
	envMap := map[string]string{
		"LISTEN_ADDR":                            "web.listen-address",
		"WEB_LISTEN_ADDRESS":                     "web.listen-address",
		"LOG_LEVEL":                              "log.level",
		"LOG_FILE":                               "log.file",
		"TELEMETRY_PATH":                         "telemetry.path",
		"URL_SECURE_PATH_NODE_SMARTCTL_EXPORTER": "telemetry.path",
		"SMARTCTL_PATH":                          "smartctl.path",
		"SMARTMONTOOLS_VERSION":                  "smartctl.installer-version",
		"FW_PROFILE":                             "firewall.profile",
		"FW_ENABLED":                             "firewall.enabled",
	}
	for envKey, flatKey := range envMap {
		if v, ok := os.LookupEnv(envKey); ok && strings.TrimSpace(v) != "" {
			flat[flatKey] = strings.TrimSpace(v)
		}
	}
}

func flattenYAML(b []byte) (map[string]string, error) {
	var root map[string]any
	if err := yaml.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, child := range t {
				key := k
				if prefix != "" {
					key = prefix + "." + k
				}
				walk(key, child)
			}
		case map[any]any:
			for k, child := range t {
				ks := fmt.Sprint(k)
				key := ks
				if prefix != "" {
					key = prefix + "." + ks
				}
				walk(key, child)
			}
		default:
			out[normalizeKey(prefix)] = fmt.Sprint(t)
		}
	}
	walk("", root)
	return out, nil
}

func NormalizeTelemetryPath(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "/metrics"
	}
	s = strings.ReplaceAll(s, `\`, `/`)
	s = strings.Trim(s, "/")
	if strings.HasSuffix(strings.ToLower(s), "/metrics") {
		s = strings.TrimSuffix(s, "/metrics")
		s = strings.TrimSuffix(s, "/Metrics")
		s = strings.Trim(s, "/")
	}
	if s == "" || strings.EqualFold(s, "metrics") {
		return "/metrics"
	}
	if strings.HasPrefix(raw, "/") && strings.Contains(raw, "/metrics") {
		return raw
	}
	parts := strings.Split(s, "/")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			return "/" + p + "/metrics"
		}
	}
	return "/metrics"
}

func normalizeKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	k = strings.ReplaceAll(k, "_", "-")
	return k
}

func normalizeProfile(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "domain", "private", "public", "any":
		return p
	default:
		return "any"
	}
}

func get(m map[string]string, key, def string) string {
	if v, ok := m[normalizeKey(key)]; ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func getBool(m map[string]string, key string, def bool) bool {
	v, ok := m[normalizeKey(key)]
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
