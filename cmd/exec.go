package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/kangthink/infra-god-cli/internal/output"
	sshclient "github.com/kangthink/infra-god-cli/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	execDryRunFlag bool
	execSerialFlag bool
	execSilentFlag bool
	execYesFlag    bool
)

// Dangerous command patterns
var blockedPatterns = []string{
	"rm -rf /",
	"rm -rf /*",
	"mkfs",
	"dd if=",
	":(){ :|:& };:",
	"> /dev/sda",
	"chmod -R 777 /",
}

var confirmPatterns = []string{
	"reboot",
	"shutdown",
	"poweroff",
	"kill -9",
	"kill -KILL",
	"systemctl stop",
	"systemctl disable",
	"docker rm",
	"docker stop",
	"docker system prune",
}

var warnPatterns = []string{
	"apt upgrade",
	"apt-get upgrade",
	"pip install",
	"chmod -R",
	"chown -R",
}

var execCmd = &cobra.Command{
	Use:   "exec <command> [server...]",
	Short: "Execute command on servers in parallel",
	Long:  "Run a shell command across multiple servers concurrently.",
	Args:  cobra.MinimumNArgs(1),
	Run:   runExec,
}

func init() {
	execCmd.Flags().BoolVar(&sudoFlag, "sudo", false, "execute with sudo")
	execCmd.Flags().BoolVar(&execDryRunFlag, "dry-run", false, "show targets without executing")
	execCmd.Flags().BoolVar(&execSerialFlag, "serial", false, "execute serially instead of parallel")
	execCmd.Flags().BoolVar(&execSilentFlag, "silent", false, "output only, no headers")
	execCmd.Flags().BoolVar(&execYesFlag, "yes", false, "skip confirmation prompts")
	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) {
	command := args[0]
	serverArgs := args[1:]

	// Safety check
	risk := checkCommandRisk(command)
	switch risk {
	case "blocked":
		fmt.Printf("\n %s %s\n", output.BoldRed("BLOCKED:"), "destructive command detected")
		fmt.Printf("    Pattern: %s\n", command)
		fmt.Printf("    Use --force to override (not recommended)\n\n")
		return
	case "confirm":
		if !execYesFlag {
			fmt.Printf("\n %s %s\n", output.Yellow("⚠️  CONFIRM:"), command)
			fmt.Printf("    This is a medium-risk operation. Proceed? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(answer) != "y" {
				fmt.Println("    Cancelled.")
				return
			}
		}
	case "warn":
		fmt.Printf(" %s %s\n", output.Yellow("⚠️"), command)
	}

	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	targets, err := resolveTargets(cfg, servers, serverArgs)
	if err != nil {
		fatalErr("resolve targets", err)
	}

	if len(targets) == 0 {
		fmt.Println("No targets matched.")
		return
	}

	// Dry run
	if execDryRunFlag {
		fmt.Printf("\n DRY RUN: %s\n", output.Bold(command))
		fmt.Printf(" Targets (%d):\n", len(targets))
		for _, srv := range targets {
			fmt.Printf("  • %s (%s)\n", srv.Name, srv.PrimaryIP())
		}
		fmt.Println()
		return
	}

	if !execSilentFlag {
		label := fmt.Sprintf("--all (%d servers)", len(targets))
		if groupFlag != "" {
			label = fmt.Sprintf("group: %s (%d servers)", groupFlag, len(targets))
		} else if len(serverArgs) > 0 {
			label = strings.Join(serverArgs, ", ")
		}
		fmt.Println(output.Header(fmt.Sprintf("EXEC: %s    %s", command, label)))
	}

	// Execute
	client := newSSHClient()
	var results []*sshclient.Result

	if execSerialFlag {
		for _, srv := range targets {
			var r *sshclient.Result
			if sudoFlag {
				r = client.RunSudo(srv, command)
			} else {
				r = client.Run(srv, command)
			}
			results = append(results, r)
			printExecResult(r)
		}
	} else {
		if sudoFlag {
			results = sshclient.RunParallelSudo(client, targets, command, parallelFlag)
		} else {
			results = sshclient.RunParallel(client, targets, command, parallelFlag)
		}
		for _, r := range results {
			printExecResult(r)
		}
	}

	// Summary
	if !execSilentFlag && len(results) > 1 {
		var succeeded, failed int
		var totalDuration time.Duration
		for _, r := range results {
			if r.Error == nil {
				succeeded++
			} else {
				failed++
			}
			totalDuration += r.Duration
		}
		avg := totalDuration / time.Duration(len(results))
		fmt.Printf("\n SUMMARY: %s/%d succeeded | avg %s\n\n",
			output.Green(fmt.Sprintf("%d", succeeded)),
			len(results),
			avg.Round(time.Millisecond),
		)
	}
}

func printExecResult(r *sshclient.Result) {
	if execSilentFlag {
		if r.Error == nil {
			fmt.Println(r.Output)
		}
		return
	}

	if r.Error != nil {
		fmt.Printf(" %s %s (%s) %s\n", output.Red("❌"), r.Server, r.Duration.Round(time.Millisecond), output.Red(r.Error.Error()))
		if r.Output != "" {
			fmt.Println(r.Output)
		}
	} else {
		fmt.Printf(" %s %s (%s)\n", output.Green("✅"), r.Server, r.Duration.Round(time.Millisecond))
		if r.Output != "" {
			fmt.Println(r.Output)
		}
	}
	fmt.Println()
}

func checkCommandRisk(command string) string {
	lower := strings.ToLower(command)
	for _, p := range blockedPatterns {
		if strings.Contains(lower, p) {
			return "blocked"
		}
	}
	for _, p := range confirmPatterns {
		if strings.Contains(lower, p) {
			return "confirm"
		}
	}
	for _, p := range warnPatterns {
		if strings.Contains(lower, p) {
			return "warn"
		}
	}
	return "safe"
}
