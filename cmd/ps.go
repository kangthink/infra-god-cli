package cmd

import (
	"fmt"
	"strings"

	"github.com/kangthink/infra-god-cli/internal/collector"
	"github.com/kangthink/infra-god-cli/internal/output"
	sshclient "github.com/kangthink/infra-god-cli/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	psSortFlag string
	psTopFlag  int
)

var psCmd = &cobra.Command{
	Use:   "ps [server...]",
	Short: "Show top processes by CPU or memory",
	Run:   runPs,
}

func init() {
	psCmd.Flags().StringVar(&psSortFlag, "sort", "cpu", "sort by: cpu, mem")
	psCmd.Flags().IntVar(&psTopFlag, "top", 10, "number of processes to show")
	rootCmd.AddCommand(psCmd)
}

func runPs(cmd *cobra.Command, args []string) {
	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	targets, err := resolveTargets(cfg, servers, args)
	if err != nil {
		fatalErr("resolve targets", err)
	}

	client := newSSHClient()

	remoteCmd := collector.PsCmd
	if psSortFlag == "mem" {
		remoteCmd = collector.PsMemCmd
	}

	if len(targets) == 1 {
		// Single server: full output
		srv := targets[0]
		fmt.Println(output.Header(fmt.Sprintf("PROCESSES: %s (sort: %s)", srv.Name, psSortFlag)))
		result := client.Run(srv, remoteCmd)
		if result.Error != nil {
			fmt.Printf(" %s %s\n", output.Red("❌"), result.Error)
			return
		}
		fmt.Println(result.Output)
	} else {
		// Multiple servers: parallel, top N each
		fmt.Println(output.Header(fmt.Sprintf("PROCESSES (top %d, sort: %s)", psTopFlag, psSortFlag)))

		topCmd := fmt.Sprintf("ps aux --sort=-%s | head -%d", psSortFlag, psTopFlag+1)
		results := sshclient.RunParallel(client, targets, topCmd, parallelFlag)

		for i, srv := range targets {
			r := results[i]
			if r.Error != nil {
				fmt.Printf("\n %s %s: %s\n", output.Red("❌"), srv.Name, r.Error)
				continue
			}
			fmt.Printf("\n %s %s\n", output.Bold(srv.Name), output.Dim(srv.PrimaryIP()))
			lines := strings.Split(r.Output, "\n")
			for j, line := range lines {
				if j == 0 {
					fmt.Printf("  %s\n", output.Dim(line)) // header
				} else {
					fmt.Printf("  %s\n", line)
				}
			}
		}
	}
	fmt.Println()
}
