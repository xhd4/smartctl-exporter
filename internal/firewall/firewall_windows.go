//go:build windows

package firewall

import (
	"fmt"
	"os/exec"
	"strings"
)

const RulePrefix = "smartctl-exporter (TCP "
const LegacyRulePrefix = "SmartctlExporter (TCP "

func Ensure(enabled bool, profile, programPath string, port int) error {
	if !enabled {
		return nil
	}
	_ = DeleteByPrefixes(RulePrefix, LegacyRulePrefix)
	name := fmt.Sprintf("%s%d)", RulePrefix, port)
	return Add(name, programPath, port, profile)
}

func DeleteAll() error {
	return DeleteByPrefixes(RulePrefix, LegacyRulePrefix)
}

func DeleteByPrefixes(prefixes ...string) error {
	for _, p := range prefixes {
		escaped := strings.ReplaceAll(p, "'", "''")
		script := fmt.Sprintf(
			`$p='%s'; Get-NetFirewallRule -DisplayName ($p+'*') -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue | Out-Null`,
			escaped,
		)
		_ = runPS(script)
	}
	return nil
}

func Add(ruleName, exePath string, port int, profile string) error {
	name := strings.ReplaceAll(ruleName, "'", "''")
	exe := strings.ReplaceAll(exePath, "'", "''")
	psProfile := mapProfile(profile)
	script := fmt.Sprintf(
		`New-NetFirewallRule -DisplayName '%s' -Direction Inbound -Action Allow -Enabled True -Program '%s' -Protocol TCP -LocalPort %d -Profile %s | Out-Null`,
		name, exe, port, psProfile,
	)
	return runPS(script)
}

func mapProfile(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "domain":
		return "Domain"
	case "private":
		return "Private"
	case "public":
		return "Public"
	default:
		return "Any"
	}
}

func runPS(script string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
