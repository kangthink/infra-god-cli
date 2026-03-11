package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/kangthink/infra-god-cli/internal/inventory"
	sshclient "github.com/kangthink/infra-god-cli/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	cfgPath     string
	groupFlag   string
	allFlag     bool
	jsonFlag    bool
	timeoutFlag time.Duration
	parallelFlag int
	sudoFlag    bool
	verboseFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "infra-god",
	Short: "CLI tool for managing multiple servers",
	Long:  "infra-god — Monitor and manage your server fleet from the terminal.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default: ./servers.yaml)")
	rootCmd.PersistentFlags().StringVar(&groupFlag, "group", "", "target server group")
	rootCmd.PersistentFlags().BoolVar(&allFlag, "all", false, "target all active servers")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "JSON output")
	rootCmd.PersistentFlags().DurationVar(&timeoutFlag, "timeout", 10*time.Second, "SSH timeout")
	rootCmd.PersistentFlags().IntVar(&parallelFlag, "parallel", 10, "max parallel SSH connections")
	rootCmd.PersistentFlags().BoolVar(&verboseFlag, "verbose", false, "verbose output")
}

// loadInventory loads config and returns resolved servers.
func loadInventory() (*inventory.Config, map[string]*inventory.ResolvedServer, error) {
	path := cfgPath
	if path == "" {
		path = inventory.DefaultConfigPath()
	}
	return inventory.Load(path)
}

// resolveTargets resolves CLI args to target servers.
func resolveTargets(cfg *inventory.Config, servers map[string]*inventory.ResolvedServer, args []string) ([]*inventory.ResolvedServer, error) {
	if !allFlag && groupFlag == "" && len(args) == 0 {
		// Default to all active
		allFlag = true
	}
	return inventory.ResolveTargets(cfg, servers, args, groupFlag, allFlag)
}

// newSSHClient creates an SSH client with configured timeout.
func newSSHClient() *sshclient.Client {
	return sshclient.NewClient(timeoutFlag)
}

// fatalErr prints error and exits.
func fatalErr(msg string, err error) {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n", msg, err)
	os.Exit(1)
}
