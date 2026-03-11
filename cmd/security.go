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

var securityFixFlag bool

var securityCmd = &cobra.Command{
	Use:   "security [server...]",
	Short: "Security audit of servers",
	Long:  "Check SSH config, firewall, accounts, updates, and Docker security.",
	Run:   runSecurity,
}

func init() {
	securityCmd.Flags().BoolVar(&securityFixFlag, "fix", false, "show remediation suggestions")
	rootCmd.AddCommand(securityCmd)
}

func runSecurity(cmd *cobra.Command, args []string) {
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
		printSecurityDetail(client, targets[0])
	} else {
		printSecuritySummary(client, targets)
	}
}

type securityScore struct {
	server   string
	score    int
	critical int
	warnings int
	passed   int
	issues   []string
}

func printSecurityDetail(client *sshclient.Client, srv *inventory.ResolvedServer) {
	fmt.Println(output.Header(fmt.Sprintf("SECURITY AUDIT: %s (%s)", srv.Name, srv.PrimaryIP())))

	result := client.RunSudo(srv, collector.SecurityCmd)
	if result.Error != nil {
		fmt.Printf(" %s %s\n", output.Red("❌"), result.Error)
		return
	}

	score := parseSecurityOutput(srv.Name, result.Output)
	printSecurityScore(score)
}

func printSecuritySummary(client *sshclient.Client, targets []*inventory.ResolvedServer) {
	fmt.Println(output.Header("SECURITY SUMMARY"))

	results := sshclient.RunParallelSudo(client, targets, collector.SecurityCmd, parallelFlag)

	table := output.NewTable("SERVER", "SCORE", "CRITICAL", "WARNING", "TOP ISSUES")
	var totalScore int
	var validCount int

	for i, srv := range targets {
		r := results[i]
		if r.Error != nil {
			table.AddRow(srv.Name, output.Red("err"), "-", "-", r.Error.Error())
			continue
		}

		score := parseSecurityOutput(srv.Name, r.Output)
		totalScore += score.score
		validCount++

		scoreStr := fmt.Sprintf("%d/100", score.score)
		if score.score >= 80 {
			scoreStr = output.Green(scoreStr)
		} else if score.score >= 60 {
			scoreStr = output.Yellow(scoreStr)
		} else {
			scoreStr = output.Red(scoreStr)
		}

		issues := "-"
		if len(score.issues) > 0 {
			if len(score.issues) > 2 {
				issues = strings.Join(score.issues[:2], ", ") + "..."
			} else {
				issues = strings.Join(score.issues, ", ")
			}
		}

		table.AddRow(
			srv.Name,
			scoreStr,
			fmt.Sprintf("%d", score.critical),
			fmt.Sprintf("%d", score.warnings),
			issues,
		)
	}

	fmt.Println(table.Render())

	if validCount > 0 {
		avg := totalScore / validCount
		fmt.Printf(" FLEET AVG: %d/100\n\n", avg)
	}
}

func parseSecurityOutput(name string, raw string) securityScore {
	score := securityScore{
		server: name,
		score:  100,
	}

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
		case "SSH":
			if strings.Contains(line, "PermitRootLogin") && !strings.Contains(line, "no") {
				score.score -= 15
				score.warnings++
				score.issues = append(score.issues, "root login enabled")
			}
			if strings.Contains(line, "PasswordAuthentication") && strings.Contains(line, "yes") {
				score.score -= 5
				score.warnings++
				score.issues = append(score.issues, "password auth")
			}
		case "FAIL2BAN":
			if strings.Contains(line, "not_installed") || strings.Contains(line, "inactive") {
				score.score -= 10
				score.warnings++
				score.issues = append(score.issues, "no fail2ban")
			}
		case "FIREWALL":
			if strings.Contains(line, "inactive") || strings.Contains(line, "not_installed") {
				score.score -= 15
				score.critical++
				score.issues = append(score.issues, "firewall off")
			}
		case "EMPTY_PASSWORDS":
			if line != "" {
				score.score -= 20
				score.critical++
				score.issues = append(score.issues, "empty passwords: "+line)
			}
		case "UID0":
			if line != "root" {
				score.score -= 20
				score.critical++
				score.issues = append(score.issues, "extra UID 0: "+line)
			}
		case "UPDATES":
			if !strings.Contains(line, "0") && !strings.Contains(line, "no_reboot") {
				score.score -= 5
				score.warnings++
				score.issues = append(score.issues, "pending updates")
			}
			if strings.Contains(line, "reboot") && !strings.Contains(line, "no_reboot") {
				score.score -= 5
				score.warnings++
				score.issues = append(score.issues, "reboot required")
			}
		}
	}

	if score.score < 0 {
		score.score = 0
	}
	score.passed = 12 - score.critical - score.warnings
	if score.passed < 0 {
		score.passed = 0
	}

	return score
}

func printSecurityScore(score securityScore) {
	for _, issue := range score.issues {
		if score.critical > 0 {
			fmt.Printf("  %s %s\n", output.Red("🚨"), issue)
		} else {
			fmt.Printf("  %s %s\n", output.Yellow("⚠️"), issue)
		}
	}

	scoreStr := fmt.Sprintf("%d/100", score.score)
	if score.score >= 80 {
		scoreStr = output.Green(scoreStr)
	} else if score.score >= 60 {
		scoreStr = output.Yellow(scoreStr)
	} else {
		scoreStr = output.Red(scoreStr)
	}

	fmt.Printf("\n SCORE: %s | %s critical | %s warning | %s pass\n\n",
		scoreStr,
		output.Red(fmt.Sprintf("%d", score.critical)),
		output.Yellow(fmt.Sprintf("%d", score.warnings)),
		output.Green(fmt.Sprintf("%d", score.passed)),
	)
}
