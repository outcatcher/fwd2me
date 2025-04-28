package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	portsStr = flag.String("ports", "", "Comma-separated list of ports to forward (REQUIRED)")
	proto    = flag.String("proto", "TCP", "Forwarded port protocol")
	label    = flag.String("label", "fwd2me", "Label for the forwarding")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sig

		cancel()
	}()

	if err := keepForwarded(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())

		os.Exit(1)
	}

	fmt.Println("Goodbye!")
}
