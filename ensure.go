package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/outcatcher/fwd2me/forwarder"
)

var (
	duration      = time.Hour
	retryDuration = time.Second
)

// keepForwarded makes sure ports are forwarded as long as possible without using a long lease.
func keepForwarded(ctx context.Context) error {
	if *portsStr == "" {
		return fmt.Errorf("no ports given")
	}

	portSlice := strings.Split(*portsStr, ",")

	ports := make([]uint16, 0, len(portSlice))

	for _, portStr := range portSlice {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			continue
		}

		ports = append(ports, uint16(port))
	}

	opts := forwarder.ForwardOpts{
		LeaseDuration: duration,
		Protocol:      *proto,
		RemoteHost:    "",
		ProgramName:   *label,
		Ports:         ports,
	}

	ticker := time.NewTimer(duration)

	forwarder := new(forwarder.Forwarder)

	if err := forwarder.Init(ctx); err != nil {
		return fmt.Errorf("error starting forwardings: %w", err)
	}

	if err := forwarder.ForwardPorts(ctx, opts); err != nil {
		return fmt.Errorf("error starting forwardings: %w", err)
	}

	shutdownCtx := context.WithoutCancel(ctx)

	for {
		select {
		case <-ticker.C:
			if err := forwarder.ForwardPorts(ctx, opts); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err.Error(), "retry in", retryDuration)

				ticker.Reset(retryDuration)

				continue
			}

			ticker.Reset(duration)
		case <-ctx.Done():
			if err := forwarder.StopAllForwarding(shutdownCtx); err != nil {
				return fmt.Errorf("error stopping forwarding: %w", err)
			}

			return nil
		}
	}
}
