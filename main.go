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
	proto = flag.String("proto", "TCP", "Forwarded port protocol")
	label = flag.String("label", "fwd2me", "Label for the forwarding")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s [options] [port1 port2 ...]:\n", os.Args[0])
		flag.PrintDefaults()
	}

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
