// Source: https://github.com/huin/goupnp/blob/main/GUIDE.md
// modifed by Anton Kachurin

package forwarder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
	"golang.org/x/sync/errgroup"
)

type routerClient interface {
	AddPortMappingCtx(
		ctx context.Context,
		NewRemoteHost string,
		NewExternalPort uint16,
		NewProtocol string,
		NewInternalPort uint16,
		NewInternalClient string,
		NewEnabled bool,
		NewPortMappingDescription string,
		NewLeaseDuration uint32,
	) (err error)

	GetGenericPortMappingEntryCtx(
		ctx context.Context,
		NewPortMappingIndex uint16,
	) (NewRemoteHost string,
		NewExternalPort uint16,
		NewProtocol string,
		NewInternalPort uint16,
		NewInternalClient string,
		NewEnabled bool,
		NewPortMappingDescription string,
		NewLeaseDuration uint32,
		err error)

	DeletePortMappingCtx(
		ctx context.Context,
		NewRemoteHost string,
		NewExternalPort uint16,
		NewProtocol string,
	) (err error)

	GetExternalIPAddressCtx(ctx context.Context) (
		NewExternalIPAddress string,
		err error,
	)

	LocalAddr() net.IP
}

func pickRouterClient(ctx context.Context) (routerClient, error) {
	tasks, _ := errgroup.WithContext(ctx)
	// Request each type of client in parallel, and return what is found.
	var ip1Clients []*internetgateway2.WANIPConnection1
	tasks.Go(func() error {
		var err error
		ip1Clients, _, err = internetgateway2.NewWANIPConnection1Clients()
		return err
	})
	var ip2Clients []*internetgateway2.WANIPConnection2
	tasks.Go(func() error {
		var err error
		ip2Clients, _, err = internetgateway2.NewWANIPConnection2Clients()
		return err
	})
	var ppp1Clients []*internetgateway2.WANPPPConnection1
	tasks.Go(func() error {
		var err error
		ppp1Clients, _, err = internetgateway2.NewWANPPPConnection1Clients()
		return err
	})

	if err := tasks.Wait(); err != nil {
		return nil, err
	}

	// Trivial handling for where we find exactly one device to talk to, you
	// might want to provide more flexible handling than this if multiple
	// devices are found.
	switch {
	case len(ip2Clients) == 1:
		return ip2Clients[0], nil
	case len(ip1Clients) == 1:
		return ip1Clients[0], nil
	case len(ppp1Clients) == 1:
		return ppp1Clients[0], nil
	default:
		return nil, errors.New("multiple or no services found")
	}
}

type ForwardedPort struct {
	InternalPort uint16
	ExternalPort uint16
	Protocol     string
}

type ForwardOpts struct {
	RemoteHost    string
	ProgramName   string
	Ports         []*ForwardedPort
	LeaseDuration time.Duration
}

type forwardedPort struct {
	ForwardedPort

	// ForwardedPort lacking fields
	remoteHost    string
	leaseDuration uint32
	label         string
	enabled       bool
}

type Forwarder struct {
	client routerClient
	logger *slog.Logger

	existingForwarding map[forwardedPort]struct{}
}

type leveler struct{}

func (l *leveler) Level() slog.Level {
	if os.Getenv("DEBUG") != "" {
		return slog.LevelDebug
	}

	return slog.LevelInfo
}

func (f *Forwarder) Init(ctx context.Context) error {
	hdl := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: new(leveler)})

	f.logger = slog.New(hdl)

	client, err := pickRouterClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to init forwarder: %w", err)
	}

	f.client = client
	f.existingForwarding = make(map[forwardedPort]struct{})

	return nil
}

// StopAllForwarding shuts down all existing forwardings.
func (f *Forwarder) StopAllForwarding(ctx context.Context) error {
	f.logger.InfoContext(ctx, "Shutting down existing forwarding")

	deleted := make([]forwardedPort, 0, len(f.existingForwarding))

	var errs error

	for fwd := range f.existingForwarding {
		err := f.client.DeletePortMappingCtx(ctx, fwd.remoteHost, fwd.ExternalPort, fwd.Protocol)

		if err != nil {
			f.logger.ErrorContext(ctx, "Failed to delete port fowarding", "externalPort", fwd.ExternalPort)

			errs = errors.Join(errs, err)
		}

		deleted = append(deleted, fwd)

		f.logger.InfoContext(ctx, "Port forwarding stopped", "externalPort", fwd.ExternalPort)
	}

	for _, deletedPort := range deleted {
		delete(f.existingForwarding, deletedPort)
	}

	if errs != nil {
		return fmt.Errorf("errors stopping forwarding: %w", errs)
	}

	return nil
}
