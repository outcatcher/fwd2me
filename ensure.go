package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/outcatcher/fwd2me/forwarder"
)

const portSeparator = ":"

var (
	leaseDuration = time.Hour
	retryDuration = time.Second

	checkDuration = 10 * time.Second

	errEmptyPortList = errors.New("empty port list")
)

func parsePort(portStr, defaultProto string) (*forwarder.ForwardedPort, error) {
	parts := strings.Split(portStr, portSeparator)

	result := &forwarder.ForwardedPort{
		Protocol: defaultProto,
	}

	if len(parts) > 0 {
		local, err := strconv.ParseUint(parts[0], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("failed to parse port, local part %s is not uint16: %w", parts[0], err)
		}

		result.InternalPort = uint16(local)
		result.ExternalPort = result.InternalPort
	}

	if len(parts) > 1 {
		remote, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("failed to parse port, remote part %s is not uint16: %w", parts[1], err)
		}

		result.ExternalPort = uint16(remote)
	}

	if len(parts) > 2 {
		result.Protocol = parts[2]
	}

	return result, nil
}

// keepForwarded makes sure ports are forwarded as long as possible without using a long lease.
func keepForwarded(ctx context.Context) error {
	portSlice := flag.Args()

	ports := make([]*forwarder.ForwardedPort, 0, len(portSlice))

	for _, portStr := range portSlice {
		forwarded, err := parsePort(portStr, *proto)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse port string:", portStr)

			continue
		}

		ports = append(ports, forwarded)
	}

	if len(ports) == 0 {
		return errEmptyPortList
	}

	opts := forwarder.ForwardOpts{
		LeaseDuration: leaseDuration,
		RemoteHost:    "",
		ProgramName:   *label,
		Ports:         ports,
	}

	forwardTimer := time.NewTimer(leaseDuration)
	checkTicker := time.NewTicker(checkDuration)

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
		case <-forwardTimer.C:
			if err := forwarder.ForwardPorts(ctx, opts); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err.Error(), "retry in", retryDuration)

				forwardTimer.Reset(retryDuration)

				continue
			}

			forwardTimer.Reset(leaseDuration)
		case <-checkTicker.C:
			// make sure forwardings are not dropped due to inactivity
			err := forwarder.EnsureForwarded(ctx)

			if err != nil {

				_, _ = fmt.Fprintln(os.Stderr, err.Error())
			}
		case <-ctx.Done():
			if err := forwarder.StopAllForwarding(shutdownCtx); err != nil {
				return fmt.Errorf("error stopping forwarding: %w", err)
			}

			return nil
		}
	}
}
