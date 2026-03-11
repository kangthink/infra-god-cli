package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/kangthink/infra-god-cli/internal/inventory"
	"github.com/kangthink/infra-god-cli/internal/output"
	"github.com/spf13/cobra"
)

var cpCmd = &cobra.Command{
	Use:   "cp <src> <dst>",
	Short: "Copy files between local and remote servers",
	Long: `Transfer files using SCP.

Examples:
  infra-god cp ./script.sh main1:/tmp/           # local → remote
  infra-god cp main1:/var/log/syslog ./           # remote → local
  infra-god cp ./script.sh --all:/tmp/            # local → all servers
  infra-god cp ./script.sh --group workers:/tmp/  # local → group`,
	Args: cobra.ExactArgs(2),
	Run:  runCp,
}

func init() {
	rootCmd.AddCommand(cpCmd)
}

func runCp(cmd *cobra.Command, args []string) {
	src := args[0]
	dst := args[1]

	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	// Determine direction and targets
	srcServer, srcPath := parseSCPArg(src)
	dstServer, dstPath := parseSCPArg(dst)

	if srcServer != "" && dstServer != "" {
		fatalErr("invalid", fmt.Errorf("cannot copy between two remote servers directly"))
	}

	// Multi-server push: --all:/path or --group name:/path
	if strings.HasPrefix(dstServer, "--all") {
		targets, err := resolveTargets(cfg, servers, nil)
		if err != nil {
			fatalErr("resolve targets", err)
		}
		allFlag = false // reset
		pushToMultiple(targets, srcPath, dstPath)
		return
	}
	if strings.HasPrefix(dstServer, "--group ") {
		groupName := strings.TrimPrefix(dstServer, "--group ")
		groupFlag = groupName
		targets, err := resolveTargets(cfg, servers, nil)
		if err != nil {
			fatalErr("resolve targets", err)
		}
		groupFlag = "" // reset
		pushToMultiple(targets, srcPath, dstPath)
		return
	}

	if srcServer == "" && dstServer != "" {
		// Local → Remote
		srv, ok := servers[dstServer]
		if !ok {
			fatalErr("unknown server", fmt.Errorf("%s", dstServer))
		}
		fmt.Println(output.Header(fmt.Sprintf("CP: %s → %s:%s", srcPath, srv.Name, dstPath)))
		err := scpPush(srv.SSHUser, srv.PrimaryIP(), srv.SSHPort, srv.AuthType, srv.KeyPath, srv.Password, srcPath, dstPath)
		if err != nil {
			fmt.Printf(" %s %s\n\n", output.Red("❌"), err)
		} else {
			fmt.Printf(" %s Copied to %s:%s\n\n", output.Green("✅"), srv.Name, dstPath)
		}
	} else if srcServer != "" && dstServer == "" {
		// Remote → Local
		srv, ok := servers[srcServer]
		if !ok {
			fatalErr("unknown server", fmt.Errorf("%s", srcServer))
		}
		fmt.Println(output.Header(fmt.Sprintf("CP: %s:%s → %s", srv.Name, srcPath, dstPath)))
		err := scpPull(srv.SSHUser, srv.PrimaryIP(), srv.SSHPort, srv.AuthType, srv.KeyPath, srv.Password, srcPath, dstPath)
		if err != nil {
			fmt.Printf(" %s %s\n\n", output.Red("❌"), err)
		} else {
			fmt.Printf(" %s Copied from %s:%s\n\n", output.Green("✅"), srv.Name, srcPath)
		}
	} else {
		// Local → Local (just use regular cp)
		fatalErr("invalid", fmt.Errorf("at least one argument must be remote (server:path)"))
	}
}

func pushToMultiple(targets []*inventory.ResolvedServer, srcPath, dstPath string) {
	fmt.Println(output.Header(fmt.Sprintf("CP: %s → %d servers:%s", srcPath, len(targets), dstPath)))

	var wg sync.WaitGroup
	results := make([]struct {
		name string
		err  error
	}, len(targets))

	sem := make(chan struct{}, parallelFlag)
	for i, srv := range targets {
		wg.Add(1)
		go func(idx int, s *inventory.ResolvedServer) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx].name = s.Name
			results[idx].err = scpPush(s.SSHUser, s.PrimaryIP(), s.SSHPort, s.AuthType, s.KeyPath, s.Password, srcPath, dstPath)
		}(i, srv)
	}
	wg.Wait()

	var ok, fail int
	for _, r := range results {
		if r.err != nil {
			fmt.Printf(" %s %s: %s\n", output.Red("❌"), r.name, r.err)
			fail++
		} else {
			fmt.Printf(" %s %s\n", output.Green("✅"), r.name)
			ok++
		}
	}
	fmt.Printf("\n %d/%d succeeded\n\n", ok, len(targets))
}

func parseSCPArg(arg string) (server, path string) {
	// Handle --all:/path and --group name:/path
	if strings.HasPrefix(arg, "--") {
		idx := strings.Index(arg, ":")
		if idx > 0 {
			return arg[:idx], arg[idx+1:]
		}
		return "", arg
	}
	// Handle server:/path
	idx := strings.Index(arg, ":")
	if idx > 0 && !strings.HasPrefix(arg, "/") && !strings.HasPrefix(arg, ".") {
		return arg[:idx], arg[idx+1:]
	}
	return "", arg
}

func scpPush(user, ip string, port int, authType, keyPath, password, src, dst string) error {
	args := buildSCPArgs(user, ip, port, authType, keyPath)
	args = append(args, src, fmt.Sprintf("%s@%s:%s", user, ip, dst))
	return runSCP(authType, password, args)
}

func scpPull(user, ip string, port int, authType, keyPath, password, src, dst string) error {
	args := buildSCPArgs(user, ip, port, authType, keyPath)
	args = append(args, fmt.Sprintf("%s@%s:%s", user, ip, src), dst)
	return runSCP(authType, password, args)
}

func buildSCPArgs(user, ip string, port int, authType, keyPath string) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		"-P", fmt.Sprintf("%d", port),
	}
	if authType == "key" {
		expandedKey := keyPath
		if strings.HasPrefix(expandedKey, "~/") {
			home, _ := os.UserHomeDir()
			expandedKey = home + expandedKey[1:]
		}
		args = append(args, "-i", expandedKey)
	}
	return args
}

func runSCP(authType, password string, args []string) error {
	if authType == "password" && password != "" {
		// Use sshpass
		fullArgs := append([]string{"-p", password, "scp"}, args...)
		cmd := exec.Command("sshpass", fullArgs...)
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	cmd := exec.Command("scp", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
