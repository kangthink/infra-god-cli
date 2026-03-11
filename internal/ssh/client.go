package ssh

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/kangthink/infra-god-cli/internal/inventory"
	"golang.org/x/crypto/ssh"
)

// Result holds the output of an SSH command execution.
type Result struct {
	Server   string
	IP       string
	Output   string
	Error    error
	Duration time.Duration
}

// Client manages SSH connections to servers.
type Client struct {
	Timeout time.Duration
}

// NewClient creates a new SSH client with the given timeout.
func NewClient(timeout time.Duration) *Client {
	return &Client{Timeout: timeout}
}

// Run executes a command on a server, trying primary IP first, then fallback.
// Retries up to 2 times on auth failures (intermittent SSH issues).
func (c *Client) Run(srv *inventory.ResolvedServer, command string) *Result {
	start := time.Now()
	result := &Result{Server: srv.Name}

	primaryIP := srv.PrimaryIP()
	if primaryIP == "" {
		result.Error = fmt.Errorf("no IP configured")
		result.Duration = time.Since(start)
		return result
	}

	ips := []string{primaryIP}
	if fallback := srv.FallbackIP(); fallback != "" {
		ips = append(ips, fallback)
	}

	const maxRetries = 2
	for _, ip := range ips {
		result.IP = ip
		for attempt := 0; attempt <= maxRetries; attempt++ {
			output, err := c.execute(srv, ip, command)
			if err == nil {
				result.Output = output
				result.Duration = time.Since(start)
				return result
			}
			// Only retry on auth/handshake failures (intermittent)
			errStr := err.Error()
			if attempt < maxRetries && (strings.Contains(errStr, "handshake") || strings.Contains(errStr, "authenticate")) {
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				continue
			}
			result.Error = err
			break
		}
		// If connection error (not auth), try fallback IP
		if result.Error != nil && !strings.Contains(result.Error.Error(), "handshake") {
			continue
		}
		break
	}

	result.Duration = time.Since(start)
	return result
}

// RunSudo executes a command with sudo on a server.
func (c *Client) RunSudo(srv *inventory.ResolvedServer, command string) *Result {
	if srv.AuthType == "password" && srv.Password != "" {
		sudoCmd := fmt.Sprintf("echo '%s' | sudo -S bash -c '%s'",
			strings.ReplaceAll(srv.Password, "'", "'\\''"),
			strings.ReplaceAll(command, "'", "'\\''"))
		return c.Run(srv, sudoCmd)
	}
	return c.Run(srv, "sudo "+command)
}

// Ping checks if a server is reachable via SSH.
func (c *Client) Ping(srv *inventory.ResolvedServer) *Result {
	return c.Run(srv, "echo ok")
}

// RunParallel executes a command on multiple servers concurrently.
func RunParallel(client *Client, servers []*inventory.ResolvedServer, command string, maxConcurrency int) []*Result {
	results := make([]*Result, len(servers))
	sem := make(chan struct{}, maxConcurrency)
	done := make(chan struct{})

	for i, srv := range servers {
		go func(idx int, s *inventory.ResolvedServer) {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = client.Run(s, command)
			done <- struct{}{}
		}(i, srv)
	}

	for range servers {
		<-done
	}
	return results
}

// RunParallelSudo executes a command with sudo on multiple servers concurrently.
func RunParallelSudo(client *Client, servers []*inventory.ResolvedServer, command string, maxConcurrency int) []*Result {
	results := make([]*Result, len(servers))
	sem := make(chan struct{}, maxConcurrency)
	done := make(chan struct{})

	for i, srv := range servers {
		go func(idx int, s *inventory.ResolvedServer) {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = client.RunSudo(s, command)
			done <- struct{}{}
		}(i, srv)
	}

	for range servers {
		<-done
	}
	return results
}

func (c *Client) execute(srv *inventory.ResolvedServer, ip string, command string) (string, error) {
	config, err := c.buildConfig(srv)
	if err != nil {
		return "", fmt.Errorf("auth config error: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", ip, srv.SSHPort)
	conn, err := net.DialTimeout("tcp", addr, c.Timeout)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return "", fmt.Errorf("SSH handshake failed: %w", err)
	}
	defer sshConn.Close()

	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("session failed: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		// Return output even on non-zero exit (useful for partial results)
		if len(output) > 0 {
			return strings.TrimSpace(string(output)), err
		}
		return "", fmt.Errorf("command failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Client) buildConfig(srv *inventory.ResolvedServer) (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:            srv.SSHUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.Timeout,
	}

	switch srv.AuthType {
	case "key":
		keyPath := srv.KeyPath
		if strings.HasPrefix(keyPath, "~/") {
			home, _ := os.UserHomeDir()
			keyPath = home + keyPath[1:]
		}
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read key %s: %w", keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse key: %w", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	case "password":
		if srv.Password == "" {
			return nil, fmt.Errorf("password not set (check env var)")
		}
		config.Auth = []ssh.AuthMethod{
			ssh.Password(srv.Password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = srv.Password
				}
				return answers, nil
			}),
		}
	default:
		return nil, fmt.Errorf("unknown auth type: %s", srv.AuthType)
	}

	return config, nil
}
