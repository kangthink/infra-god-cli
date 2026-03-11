package cmd

import (
	"fmt"
	"strings"

	"github.com/kangthink/infra-god-cli/internal/collector"
	"github.com/kangthink/infra-god-cli/internal/inventory"
	"github.com/kangthink/infra-god-cli/internal/output"
	sshclient "github.com/kangthink/infra-god-cli/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	healDryRunFlag     bool
	healAggressiveFlag bool
	healYesFlag        bool
)

var healCmd = &cobra.Command{
	Use:   "heal <server> <action> [target]",
	Short: "Automated server recovery",
	Long: `Perform recovery actions with pre/post validation.

Actions:
  disk-cleanup              Clean disk space (logs, cache, docker images)
  restart <service>         Restart a systemd service
  restart-docker <name>     Restart a Docker container
  security-harden           Apply basic security hardening (fail2ban, ufw, ssh)`,
	Args: cobra.MinimumNArgs(2),
	Run:  runHeal,
}

func init() {
	healCmd.Flags().BoolVar(&healDryRunFlag, "dry-run", false, "show plan without executing")
	healCmd.Flags().BoolVar(&healAggressiveFlag, "aggressive", false, "aggressive cleanup (docker volumes, snap)")
	healCmd.Flags().BoolVar(&healYesFlag, "yes", false, "skip confirmation")
	rootCmd.AddCommand(healCmd)
}

func runHeal(cmd *cobra.Command, args []string) {
	_, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	serverName := args[0]
	action := args[1]

	srv, ok := servers[serverName]
	if !ok {
		fatalErr("unknown server", fmt.Errorf("%s", serverName))
	}

	client := newSSHClient()

	switch action {
	case "disk-cleanup":
		healDiskCleanup(client, srv)
	case "restart":
		if len(args) < 3 {
			fatalErr("restart requires service name", fmt.Errorf("usage: heal %s restart <service>", serverName))
		}
		healRestart(client, srv, args[2])
	case "restart-docker":
		if len(args) < 3 {
			fatalErr("restart-docker requires container name", fmt.Errorf("usage: heal %s restart-docker <name>", serverName))
		}
		healRestartDocker(client, srv, args[2])
	case "security-harden":
		healSecurityHarden(client, srv)
	default:
		fatalErr("unknown action", fmt.Errorf("%s (available: disk-cleanup, restart, restart-docker, security-harden)", action))
	}
}

func healDiskCleanup(client *sshclient.Client, srv *inventory.ResolvedServer) {
	fmt.Println(output.Header(fmt.Sprintf("HEAL: disk-cleanup on %s", srv.Name)))

	fmt.Printf(" %s Analyzing disk usage...\n\n", output.Cyan("PLAN"))
	planResult := client.RunSudo(srv, collector.HealDiskPlanCmd)
	if planResult.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), planResult.Error)
		return
	}

	parseDiskPlan(planResult.Output)

	if healDryRunFlag {
		fmt.Printf("\n %s Use without --dry-run to execute.\n\n", output.Dim("ℹ️"))
		return
	}

	if !healYesFlag {
		fmt.Printf("\n Proceed with cleanup? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) != "y" {
			fmt.Println(" Cancelled.")
			return
		}
	}

	fmt.Printf("\n %s Cleaning...\n", output.Bold("ACTION"))
	var cleanCmd string
	if healAggressiveFlag {
		cleanCmd = collector.HealDiskCleanAggressiveCmd
	} else {
		cleanCmd = collector.HealDiskCleanCmd
	}

	cleanResult := client.RunSudo(srv, cleanCmd)
	if cleanResult.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), cleanResult.Error)
		if cleanResult.Output != "" {
			fmt.Println(cleanResult.Output)
		}
		return
	}

	fmt.Printf("\n %s\n", output.Bold("POST-CHECK"))
	for _, line := range strings.Split(cleanResult.Output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/dev/") {
			fmt.Printf("  Disk: %s\n", line)
		}
	}
	fmt.Printf(" %s Cleanup complete\n\n", output.Green("✅"))
}

func parseDiskPlan(raw string) {
	currentSection := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "===") && strings.HasSuffix(line, "===") {
			currentSection = strings.Trim(line, "=")
			continue
		}
		if line == "" {
			continue
		}
		switch currentSection {
		case "APT_CACHE":
			fmt.Printf("  [1] apt cache           %s\n", line)
		case "JOURNAL":
			fmt.Printf("  [2] journal logs        %s\n", line)
		case "TMP":
			fmt.Printf("  [3] old tmp files       %s bytes\n", line)
		case "DOCKER_IMAGES":
			fmt.Printf("  [4] docker images       %s\n", line)
		case "SNAP":
			fmt.Printf("  [5] old snap revisions  %s disabled\n", line)
		case "OLD_LOGS":
			fmt.Printf("  [6] rotated logs        %s bytes\n", line)
		case "CURRENT_DISK":
			fmt.Printf("\n  Current: %s\n", line)
		}
	}
}

func healRestart(client *sshclient.Client, srv *inventory.ResolvedServer, service string) {
	fmt.Println(output.Header(fmt.Sprintf("HEAL: restart %s on %s", service, srv.Name)))

	fmt.Printf(" %s\n", output.Bold("PRE-CHECK"))
	preCmd := fmt.Sprintf("systemctl status %s --no-pager -l 2>&1 | head -5", service)
	preResult := client.RunSudo(srv, preCmd)
	fmt.Printf("  %s\n", preResult.Output)

	if healDryRunFlag {
		fmt.Printf("\n %s Would run: systemctl restart %s\n\n", output.Dim("ℹ️"), service)
		return
	}

	fmt.Printf("\n %s systemctl restart %s\n", output.Bold("ACTION"), service)
	restartResult := client.RunSudo(srv, fmt.Sprintf("systemctl restart %s", service))
	if restartResult.Error != nil {
		fmt.Printf(" %s restart failed: %s\n", output.Red("❌"), restartResult.Error)
		return
	}

	fmt.Printf("\n %s\n", output.Bold("POST-CHECK"))
	postCmd := fmt.Sprintf("systemctl is-active %s; ss -tlnp 2>/dev/null | grep -i %s | head -3", service, service)
	postResult := client.RunSudo(srv, postCmd)
	if strings.Contains(postResult.Output, "active") {
		fmt.Printf("  %s %s is active\n", output.Green("✅"), service)
	} else {
		fmt.Printf("  %s %s: %s\n", output.Red("❌"), service, postResult.Output)
	}
	fmt.Println()
}

func healRestartDocker(client *sshclient.Client, srv *inventory.ResolvedServer, container string) {
	fmt.Println(output.Header(fmt.Sprintf("HEAL: restart-docker %s on %s", container, srv.Name)))

	fmt.Printf(" %s\n", output.Bold("PRE-CHECK"))
	preResult := client.Run(srv, fmt.Sprintf("docker inspect --format '{{.State.Status}}' %s 2>&1", container))
	fmt.Printf("  Container status: %s\n", preResult.Output)

	if healDryRunFlag {
		fmt.Printf("\n %s Would run: docker restart %s\n\n", output.Dim("ℹ️"), container)
		return
	}

	fmt.Printf("\n %s docker restart %s\n", output.Bold("ACTION"), container)
	restartResult := client.Run(srv, fmt.Sprintf("docker restart %s", container))
	if restartResult.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), restartResult.Error)
		return
	}

	fmt.Printf("\n %s\n", output.Bold("POST-CHECK"))
	postResult := client.Run(srv, fmt.Sprintf("docker inspect --format '{{.State.Status}} {{.State.Health.Status}}' %s 2>&1", container))
	fmt.Printf("  %s %s: %s\n", output.Green("✅"), container, postResult.Output)
	fmt.Println()
}

func healSecurityHarden(client *sshclient.Client, srv *inventory.ResolvedServer) {
	fmt.Println(output.Header(fmt.Sprintf("HEAL: security-harden on %s", srv.Name)))

	fmt.Printf(" %s\n", output.Bold("PRE-CHECK"))
	preResult := client.RunSudo(srv, `
echo "fail2ban: $(systemctl is-active fail2ban 2>/dev/null || echo not_installed)"
echo "ufw: $(ufw status 2>/dev/null | head -1 || echo not_installed)"
echo "root_login: $(grep '^PermitRootLogin' /etc/ssh/sshd_config 2>/dev/null || echo 'not set')"
`)
	if preResult.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), preResult.Error)
		return
	}
	for _, line := range strings.Split(preResult.Output, "\n") {
		if line != "" {
			fmt.Printf("  %s\n", line)
		}
	}

	fmt.Printf("\n Actions:\n")
	fmt.Printf("  [1] Install & enable fail2ban\n")
	fmt.Printf("  [2] Enable UFW firewall (allow SSH)\n")
	fmt.Printf("  [3] Disable root SSH login\n")
	fmt.Printf("  [4] Set MaxAuthTries to 5\n")

	if healDryRunFlag {
		fmt.Printf("\n %s Use without --dry-run to execute.\n\n", output.Dim("ℹ️"))
		return
	}

	if !healYesFlag {
		fmt.Printf("\n Proceed with security hardening? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) != "y" {
			fmt.Println(" Cancelled.")
			return
		}
	}

	fmt.Printf("\n %s\n", output.Bold("ACTION"))
	hardenResult := client.RunSudo(srv, collector.HealSecurityHardenCmd)
	if hardenResult.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), hardenResult.Error)
		if hardenResult.Output != "" {
			fmt.Println(hardenResult.Output)
		}
		return
	}

	for _, line := range strings.Split(hardenResult.Output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "===") {
			continue
		}
		if strings.Contains(line, "installed") || strings.Contains(line, "enabled") || strings.Contains(line, "hardened") {
			fmt.Printf("  %s %s\n", output.Green("✅"), line)
		} else if strings.Contains(line, "already") {
			fmt.Printf("  %s %s\n", output.Dim("✅"), line)
		}
	}

	fmt.Printf("\n %s Security hardening complete\n\n", output.Green("✅"))
}
