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

var usersAuditFlag bool

var usersCmd = &cobra.Command{
	Use:   "users [server...]",
	Short: "Show user accounts and permissions",
	Long:  "Display logged-in users, accounts, sudoers, and SSH config.",
	Run:   runUsers,
}

func init() {
	usersCmd.Flags().BoolVar(&usersAuditFlag, "audit", false, "security audit mode")
	rootCmd.AddCommand(usersCmd)
}

func runUsers(cmd *cobra.Command, args []string) {
	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	targets, err := resolveTargets(cfg, servers, args)
	if err != nil {
		fatalErr("resolve targets", err)
	}

	client := newSSHClient()

	if len(targets) == 1 {
		// Detailed view for single server
		printUsersDetail(client, targets[0])
	} else {
		// Summary view for multiple servers
		printUsersSummary(client, targets)
	}
}

func printUsersDetail(client *sshclient.Client, srv *inventory.ResolvedServer) {
	fmt.Println(output.Header(fmt.Sprintf("USERS: %s (%s)", srv.Name, srv.PrimaryIP())))

	result := client.Run(srv, collector.UsersCmd)
	if result.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), result.Error)
		return
	}

	currentSection := ""
	for _, line := range strings.Split(result.Output, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "===") && strings.HasSuffix(line, "===") {
			currentSection = strings.Trim(line, "=")
			switch currentSection {
			case "LOGGED_IN":
				fmt.Println(output.SubHeader("LOGGED IN"))
			case "ACCOUNTS":
				fmt.Println(output.SubHeader("ACCOUNTS"))
				fmt.Printf("  %-15s %-8s %-25s %s\n",
					output.Bold("USER"), output.Bold("UID"), output.Bold("SHELL"), output.Bold("HOME"))
			case "SUDOERS":
				fmt.Println(output.SubHeader("SUDOERS"))
			case "SSH_CONFIG":
				fmt.Println(output.SubHeader("SSH CONFIG"))
			}
			continue
		}

		if line == "" {
			continue
		}

		switch currentSection {
		case "LOGGED_IN":
			fmt.Printf("  %s\n", line)
		case "ACCOUNTS":
			parts := strings.SplitN(line, ":", 4)
			if len(parts) >= 4 {
				fmt.Printf("  %-15s %-8s %-25s %s\n", parts[0], parts[1], parts[2], parts[3])
			}
		case "SUDOERS":
			if line != "" {
				fmt.Printf("  %s %s\n", output.Green("✅"), line)
			}
		case "SSH_CONFIG":
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
}

func printUsersSummary(client *sshclient.Client, targets []*inventory.ResolvedServer) {
	fmt.Println(output.Header("USER SUMMARY"))

	results := sshclient.RunParallel(client, targets, collector.UsersCmd, parallelFlag)

	table := output.NewTable("SERVER", "LOGGED", "ACCOUNTS", "SUDOERS")
	for i, srv := range targets {
		r := results[i]
		if r.Error != nil {
			table.AddRow(srv.Name, output.Red("error"), "-", "-")
			continue
		}

		var logged, accounts int
		var sudoers string
		currentSection := ""

		for _, line := range strings.Split(r.Output, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "===") {
				currentSection = strings.Trim(line, "=")
				continue
			}
			if line == "" {
				continue
			}
			switch currentSection {
			case "LOGGED_IN":
				logged++
			case "ACCOUNTS":
				accounts++
			case "SUDOERS":
				sudoers = line
			}
		}

		table.AddRow(
			srv.Name,
			fmt.Sprintf("%d", logged),
			fmt.Sprintf("%d", accounts),
			sudoers,
		)
	}
	fmt.Println(table.Render())
}
