package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kangthink/infra-god-cli/internal/webui"
	"github.com/spf13/cobra"
)

var (
	serveAddr             string
	serveStatusRefresh    time.Duration
	serveContainerRefresh time.Duration
	serveDetailsRefresh   time.Duration
	serveFoldersRefresh   time.Duration
	servePrintURL         bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run a read-only LAN web UI of fleet status",
	Long: `Start an HTTP server that exposes a read-only view of the fleet's status
and Docker containers. Designed for LAN-only access — no auth by default.

Background pollers refresh status every --refresh interval, so SSH load
on monitored servers stays constant regardless of how many people view
the UI.`,
	Run: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", "0.0.0.0:9998", "bind address (host:port)")
	serveCmd.Flags().DurationVar(&serveStatusRefresh, "refresh", 30*time.Second, "status poll interval")
	serveCmd.Flags().DurationVar(&serveContainerRefresh, "container-refresh", 60*time.Second, "containers poll interval")
	serveCmd.Flags().DurationVar(&serveDetailsRefresh, "details-refresh", 120*time.Second, "ports/mounts poll interval")
	serveCmd.Flags().DurationVar(&serveFoldersRefresh, "folders-refresh", 5*time.Minute, "top-level folder size scan interval (du-based, slower)")
	serveCmd.Flags().BoolVar(&servePrintURL, "print-url", true, "print listening URL on start")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) {
	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	cache := webui.New(cfg, servers, newSSHClient(), parallelFlag, serveStatusRefresh, serveContainerRefresh, serveDetailsRefresh, serveFoldersRefresh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cache.Start(ctx)

	srv := &http.Server{
		Addr:              serveAddr,
		Handler:           webui.NewHandler(cache),
		ReadHeaderTimeout: 5 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if servePrintURL {
			fmt.Printf("infra-god webui listening on http://%s\n", serveAddr)
			fmt.Printf("  refresh interval: %s\n", serveStatusRefresh)
			fmt.Printf("  endpoints: /api/status  /api/alerts  /healthz\n")
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-sigCh
	fmt.Println("\nshutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
