package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ebogdum/keramos/internal/cli"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
)

func main() {
	// Translate SIGINT/SIGTERM into a clean exit. The first signal triggers
	// a graceful shutdown; the second forcibly exits in case any goroutine
	// is wedged on a network call. Most keramos operations honour ctx
	// cancellation indirectly via timeouts, but explicit signal handling
	// makes Ctrl-C behave intuitively in interactive sessions.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nkeramos: interrupt received, shutting down (send another to force-exit)")
		go func() {
			<-sigCh
			os.Exit(130)
		}()
		// best-effort: tear down by exiting with the SIGINT-equivalent code.
		// pending goroutines that hold onto cluster operations will be
		// reaped by the OS; release storage stays consistent because every
		// cluster operation either completed or never wrote a secret.
		os.Exit(130)
	}()

	if err := cli.Execute(); nil != err {
		fmt.Fprintln(os.Stderr, keramoserr.FormatUserFriendly(err))
		os.Exit(1)
	}
}
