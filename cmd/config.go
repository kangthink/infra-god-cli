package cmd

import (
	"fmt"
	"strings"

	"github.com/kangthink/infra-god-cli/internal/inventory"
	"github.com/kangthink/infra-god-cli/internal/output"
	sshclient "github.com/kangthink/infra-god-cli/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	configHostFlag     string
	configWirelessFlag string
	configRoleFlag     string
	configUserFlag     string
	configAuthFlag     string
	configKeyFlag      string
	configDescFlag     string
	configStatusFlag   string
)

var configCmd = &cobra.Command{
	Use:   "config <operation>",
	Short: "Manage server inventory",
	Long:  "List, add, edit, remove servers and test connectivity.",
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all servers",
	Run:   runConfigList,
}

var configTestCmd = &cobra.Command{
	Use:   "test [server...]",
	Short: "Test SSH connectivity",
	Run:   runConfigTest,
}

var configAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new server",
	Long: `Add a server to the inventory.

Examples:
  infra-god config add worker4 --host 10.0.1.30 --role worker
  infra-god config add gpu1 --host 10.0.1.50 --wireless 10.0.2.50 --role machine --auth key --key ~/.ssh/gpu1`,
	Args: cobra.ExactArgs(1),
	Run:  runConfigAdd,
}

var configEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a server",
	Long: `Edit server properties.

Examples:
  infra-god config edit gpu2 --host 10.0.1.51
  infra-god config edit worker1 --user deploy --auth key --key ~/.ssh/worker1`,
	Args: cobra.ExactArgs(1),
	Run:  runConfigEdit,
}

var configRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a server",
	Args:  cobra.ExactArgs(1),
	Run:   runConfigRemove,
}

func init() {
	// Add flags
	for _, cmd := range []*cobra.Command{configAddCmd, configEditCmd} {
		cmd.Flags().StringVar(&configHostFlag, "host", "", "server IP (wired)")
		cmd.Flags().StringVar(&configWirelessFlag, "wireless", "", "wireless IP")
		cmd.Flags().StringVar(&configRoleFlag, "role", "", "server role (main, machine, worker)")
		cmd.Flags().StringVar(&configUserFlag, "user", "", "SSH user")
		cmd.Flags().StringVar(&configAuthFlag, "auth", "", "auth type (password, key)")
		cmd.Flags().StringVar(&configKeyFlag, "key", "", "SSH key path")
		cmd.Flags().StringVar(&configDescFlag, "desc", "", "description")
	}
	configEditCmd.Flags().StringVar(&configStatusFlag, "status", "", "status (active, stopped)")

	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configTestCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configRemoveCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigList(cmd *cobra.Command, args []string) {
	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	fmt.Println(output.Header("SERVER INVENTORY"))

	table := output.NewTable("SERVER", "IP", "ROLE", "STATUS", "SSH USER", "AUTH")
	for _, name := range sortedServerNames(cfg) {
		srv := servers[name]
		ip := srv.PrimaryIP()
		if ip == "" {
			ip = "-"
		}
		if srv.WirelessIP != "" && srv.WiredIP != "" {
			ip += fmt.Sprintf(" / %s", srv.WirelessIP)
		}

		status := output.Green("active")
		if !srv.IsActive() {
			status = output.Dim("stopped")
		}

		table.AddRow(srv.Name, ip, srv.Role, status, srv.SSHUser, srv.AuthType)
	}
	fmt.Println(table.Render())

	// Groups
	fmt.Println(output.SubHeader("GROUPS"))
	for name, members := range cfg.Groups {
		fmt.Printf("  %-12s %s\n", output.Bold(name), strings.Join(members, ", "))
	}
	fmt.Println()
}

func runConfigTest(cmd *cobra.Command, args []string) {
	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	targets, err := resolveTargets(cfg, servers, args)
	if err != nil {
		fatalErr("resolve targets", err)
	}

	fmt.Println(output.Header("CONNECTION TEST"))

	client := newSSHClient()

	// Test all in parallel
	results := sshclient.RunParallel(client, targets, "echo ok", parallelFlag)

	var passed, failed int
	for i, srv := range targets {
		r := results[i]
		if r.Error != nil {
			fmt.Printf(" %s %-15s %-18s %s\n",
				output.Red("❌"), srv.Name, srv.PrimaryIP(),
				output.Red(r.Error.Error()))
			failed++

			// Try fallback IP
			if srv.FallbackIP() != "" {
				fmt.Printf("    → fallback %s: ", srv.FallbackIP())
				// Already tried in client.Run, so just report
				fmt.Println(output.Yellow("also failed"))
			}
		} else {
			fmt.Printf(" %s %-15s %-18s %s\n",
				output.Green("✅"), srv.Name, r.IP,
				output.Dim(r.Duration.Round(1e6).String()))
			passed++
		}
	}
	fmt.Printf("\n %d/%d connected\n\n", passed, len(targets))
}

func runConfigAdd(cmd *cobra.Command, args []string) {
	name := args[0]
	cfg, _, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	if _, exists := cfg.Servers[name]; exists {
		fatalErr("server exists", fmt.Errorf("%s already exists (use 'config edit' to modify)", name))
	}

	if configHostFlag == "" {
		fatalErr("missing flag", fmt.Errorf("--host is required"))
	}

	srv := inventory.Server{
		Role: configRoleFlag,
	}

	// Set host
	if configWirelessFlag != "" {
		srv.Host = map[string]interface{}{
			"wired":    configHostFlag,
			"wireless": configWirelessFlag,
		}
	} else {
		srv.Host = configHostFlag
	}

	if configUserFlag != "" {
		srv.SSHUser = configUserFlag
	}
	if configAuthFlag != "" {
		srv.Auth = &inventory.Auth{Type: configAuthFlag}
		if configKeyFlag != "" {
			srv.Auth.KeyPath = configKeyFlag
		}
	}
	if configDescFlag != "" {
		srv.Description = configDescFlag
	}

	cfg.Servers[name] = srv

	// Add to "all" group
	if all, ok := cfg.Groups["all"]; ok {
		cfg.Groups["all"] = append(all, name)
	}
	// Add to role group
	if configRoleFlag != "" {
		roleGroup := configRoleFlag + "s"
		if members, ok := cfg.Groups[roleGroup]; ok {
			cfg.Groups[roleGroup] = append(members, name)
		}
		// Add to active group
		if active, ok := cfg.Groups["active"]; ok {
			cfg.Groups["active"] = append(active, name)
		}
	}

	path := inventory.ConfigPath(cfgPath)
	if err := inventory.Save(path, cfg); err != nil {
		fatalErr("save config", err)
	}
	fmt.Printf(" %s Added %s (%s) role=%s\n\n", output.Green("✅"), name, configHostFlag, configRoleFlag)
}

func runConfigEdit(cmd *cobra.Command, args []string) {
	name := args[0]
	cfg, _, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	srv, exists := cfg.Servers[name]
	if !exists {
		fatalErr("unknown server", fmt.Errorf("%s", name))
	}

	changed := false

	if configHostFlag != "" {
		if configWirelessFlag != "" {
			srv.Host = map[string]interface{}{
				"wired":    configHostFlag,
				"wireless": configWirelessFlag,
			}
		} else {
			// Keep existing wireless if only wired changed
			switch h := srv.Host.(type) {
			case map[string]interface{}:
				h["wired"] = configHostFlag
				srv.Host = h
			default:
				srv.Host = configHostFlag
			}
		}
		changed = true
	} else if configWirelessFlag != "" {
		switch h := srv.Host.(type) {
		case map[string]interface{}:
			h["wireless"] = configWirelessFlag
			srv.Host = h
		case string:
			srv.Host = map[string]interface{}{
				"wired":    h,
				"wireless": configWirelessFlag,
			}
		}
		changed = true
	}

	if configRoleFlag != "" {
		srv.Role = configRoleFlag
		changed = true
	}
	if configUserFlag != "" {
		srv.SSHUser = configUserFlag
		changed = true
	}
	if configAuthFlag != "" {
		if srv.Auth == nil {
			srv.Auth = &inventory.Auth{}
		}
		srv.Auth.Type = configAuthFlag
		if configKeyFlag != "" {
			srv.Auth.KeyPath = configKeyFlag
		}
		changed = true
	}
	if configDescFlag != "" {
		srv.Description = configDescFlag
		changed = true
	}
	if configStatusFlag != "" {
		srv.Status = configStatusFlag
		changed = true
	}

	if !changed {
		fmt.Println(" No changes specified. Use flags like --host, --role, --user, --auth, --status")
		return
	}

	cfg.Servers[name] = srv

	path := inventory.ConfigPath(cfgPath)
	if err := inventory.Save(path, cfg); err != nil {
		fatalErr("save config", err)
	}
	fmt.Printf(" %s Updated %s\n\n", output.Green("✅"), name)
}

func runConfigRemove(cmd *cobra.Command, args []string) {
	name := args[0]
	cfg, _, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	if _, exists := cfg.Servers[name]; !exists {
		fatalErr("unknown server", fmt.Errorf("%s", name))
	}

	delete(cfg.Servers, name)

	// Remove from all groups
	for gname, members := range cfg.Groups {
		var filtered []string
		for _, m := range members {
			if m != name {
				filtered = append(filtered, m)
			}
		}
		cfg.Groups[gname] = filtered
	}

	path := inventory.ConfigPath(cfgPath)
	if err := inventory.Save(path, cfg); err != nil {
		fatalErr("save config", err)
	}
	fmt.Printf(" %s Removed %s\n\n", output.Green("✅"), name)
}

func sortedServerNames(cfg *inventory.Config) []string {
	// Use the "all" group order if available
	if all, ok := cfg.Groups["all"]; ok {
		return all
	}
	var names []string
	for name := range cfg.Servers {
		names = append(names, name)
	}
	return names
}
