package cmd

import (
	"fmt"

	"github.com/kangthink/infra-god-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	logsTailFlag   int
	logsDockerFlag string
	logsSinceFlag  string
)

var logsCmd = &cobra.Command{
	Use:   "logs <server> [service]",
	Short: "View logs from a server",
	Long:  "View systemd journal or Docker container logs.",
	Args:  cobra.MinimumNArgs(1),
	Run:   runLogs,
}

func init() {
	logsCmd.Flags().IntVar(&logsTailFlag, "tail", 50, "number of lines")
	logsCmd.Flags().StringVar(&logsDockerFlag, "docker", "", "Docker container name")
	logsCmd.Flags().StringVar(&logsSinceFlag, "since", "1h", "time range (e.g. 1h, 30m, 1d)")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) {
	_, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	serverName := args[0]
	srv, ok := servers[serverName]
	if !ok {
		fatalErr("unknown server", fmt.Errorf("%s", serverName))
	}

	client := newSSHClient()
	var remoteCmd string

	if logsDockerFlag != "" {
		// Docker container logs
		remoteCmd = fmt.Sprintf("docker logs --tail %d %s 2>&1", logsTailFlag, logsDockerFlag)
		fmt.Println(output.Header(fmt.Sprintf("DOCKER LOGS: %s → %s", srv.Name, logsDockerFlag)))
	} else if len(args) > 1 {
		// Systemd service logs
		service := args[1]
		remoteCmd = fmt.Sprintf("journalctl -u %s --no-pager -n %d --since '%s ago' 2>&1", service, logsTailFlag, logsSinceFlag)
		fmt.Println(output.Header(fmt.Sprintf("LOGS: %s → %s", srv.Name, service)))
	} else {
		// General syslog
		remoteCmd = fmt.Sprintf("journalctl --no-pager -n %d --since '%s ago' 2>&1", logsTailFlag, logsSinceFlag)
		fmt.Println(output.Header(fmt.Sprintf("LOGS: %s (system)", srv.Name)))
	}

	result := client.Run(srv, remoteCmd)
	if result.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), result.Error)
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		return
	}
	fmt.Println(result.Output)
	fmt.Println()
}
