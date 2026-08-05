//go:build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"smartctl-exporter/internal/config"
	"smartctl-exporter/internal/exporter"
	"smartctl-exporter/internal/firewall"
	"smartctl-exporter/internal/install"
)

var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var (
		showHelp    bool
		showVersion bool
		printConfig bool
		doInstall   bool
		doUninstall bool
		dryRun      bool
		configFile  string
	)

	overrides := map[string]string{}
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			showHelp = true
		case a == "--version":
			showVersion = true
		case a == "--print-config":
			printConfig = true
		case a == "--install":
			doInstall = true
		case a == "--uninstall":
			doUninstall = true
		case a == "--dry-run":
			dryRun = true
		case strings.HasPrefix(a, "--config.file="):
			configFile = strings.TrimPrefix(a, "--config.file=")
		case a == "--config.file":
			if i+1 < len(args) {
				i++
				configFile = args[i]
			}
		case strings.HasPrefix(a, "--"):
			key, val, ok := parseOverride(a, args, &i)
			if ok {
				overrides[key] = val
			}
		}
	}

	if showHelp {
		printHelp()
		return
	}
	if showVersion {
		fmt.Println(version)
		return
	}

	hostExe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exeDir := filepath.Dir(hostExe)
	if configFile == "" {
		configFile = filepath.Join(exeDir, "config.yaml")
	}
	absConfig, err := filepath.Abs(configFile)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load(exeDir, absConfig, overrides)
	if err != nil {
		log.Fatal(err)
	}

	if doUninstall {
		if err := install.Uninstall(dryRun); err != nil {
			log.Fatal(err)
		}
		return
	}
	if printConfig {
		s, err := cfg.SerializeYAML()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(s)
		return
	}
	if doInstall {
		if err := install.Install(cfg, dryRun); err != nil {
			log.Fatal(err)
		}
		return
	}

	port, err := cfg.ListenPort()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.FirewallEnabled {
		if err := firewall.Ensure(true, cfg.FirewallProfile, hostExe, port); err != nil {
			log.Printf("firewall ensure (best-effort): %v", err)
		}
	}

	expArgs := cfg.ChildArgs()
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		log.Fatal(err)
	}
	if isSvc {
		if err := svc.Run(install.ServiceName, &serviceHandler{
			args:       expArgs,
			logEnabled: cfg.LogToFile(),
			logPath:    cfg.ResolvedLogFile(),
		}); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := runForeground(expArgs, cfg); err != nil {
		log.Fatal(err)
	}
}

func parseOverride(arg string, args []string, i *int) (string, string, bool) {
	raw := strings.TrimPrefix(arg, "--")
	key, val, hasEq := strings.Cut(raw, "=")
	key = strings.ReplaceAll(key, "_", "-")
	if hasEq {
		return key, val, true
	}
	boolKeys := map[string]bool{
		"firewall.enabled": true,
		"debug.enabled":    true,
	}
	if boolKeys[key] {
		return key, "true", true
	}
	if *i+1 < len(args) && !strings.HasPrefix(args[*i+1], "--") {
		*i++
		return key, args[*i], true
	}
	return "", "", false
}

func printHelp() {
	fmt.Print(`smartctl-exporter - Windows service for S.M.A.R.T. metrics (vendored smartctl_exporter)

Usage:
  smartctl-exporter.exe [options]
  smartctl-exporter.exe --install [options] [--dry-run]
  smartctl-exporter.exe --uninstall [--dry-run]

Operational:
  --install                 Install to Program Files, register service and firewall
  --uninstall               Remove service, firewall, Program Files and ProgramData
  --dry-run                 Preview install/uninstall without changes
  --config.file PATH        YAML config (default: ./config.yaml next to exe)
  --help, -h
  --version
  --print-config

Config (CLI / YAML / env):
  --web.listen-address ADDR   env LISTEN_ADDR
  --telemetry.path PATH       env TELEMETRY_PATH
  --log.level LEVEL           env LOG_LEVEL
  --log.file PATH             env LOG_FILE
  --smartctl.path PATH        env SMARTCTL_PATH
  --smartctl.installer-version VER
  --firewall.enabled BOOL     env FW_ENABLED
  --firewall.profile NAME     env FW_PROFILE (domain|private|public|any)
`)
}

type serviceHandler struct {
	args       []string
	logEnabled bool
	logPath    string
}

func (h *serviceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.StartPending}

	if h.logEnabled {
		if err := os.MkdirAll(filepath.Dir(h.logPath), 0755); err == nil {
			if f, err := os.OpenFile(h.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				_, _ = fmt.Fprintf(f, "[%s] service starting exporter %v\n", time.Now().Format(time.DateTime), h.args)
				log.SetOutput(io.MultiWriter(f, os.Stderr))
				defer f.Close()
			}
		}
	}

	errCh := make(chan error, 1)
	go func() { errCh <- exporter.Run(h.args) }()
	s <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				log.Printf("exporter exited: %v", err)
			}
			s <- svc.Status{State: svc.Stopped}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				s <- svc.Status{State: svc.Stopped}
				os.Exit(0)
			}
		}
	}
}

func runForeground(expArgs []string, cfg config.Config) error {
	if cfg.LogToFile() {
		path := cfg.ResolvedLogFile()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, _ = fmt.Fprintf(f, "[%s] foreground exporter %v\n", time.Now().Format(time.DateTime), expArgs)
		log.SetOutput(io.MultiWriter(os.Stdout, f))
	}
	log.Printf("running in-process exporter: %v", expArgs)
	return exporter.Run(expArgs)
}
