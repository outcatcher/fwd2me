// Source: https://github.com/huin/goupnp/blob/main/GUIDE.md
// modifed by Anton Kachurin

package forwarder

import (
	"context"
	"errors"
	"fmt"
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
	remoteHost   string // ForwardOpts has no remote host
	externalPort uint16
	protocol     string
}

type Forwarder struct {
	client routerClient

	existingForwarding map[forwardedPort]struct{}
}

func (f *Forwarder) Init(ctx context.Context) error {
	client, err := pickRouterClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to init forwarder: %w", err)
	}

	f.client = client
	f.existingForwarding = make(map[forwardedPort]struct{})

	return nil
}

func (f *Forwarder) ForwardPorts(ctx context.Context, opts ForwardOpts) error {
	externalIP, err := f.client.GetExternalIPAddressCtx(ctx)
	if err != nil {
		return err
	}

	fmt.Println("Recreating forwarding from", externalIP, "to", f.client.LocalAddr().String())

	var errs error

	for _, port := range opts.Ports {
		// Try to clean up first. That's not optimal, but simplifies workflow.
		err := f.client.DeletePortMappingCtx(ctx, opts.RemoteHost, port.ExternalPort, port.Protocol)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "failed to delete before create", err.Error())

			errs = errors.Join(errs, err)
		}

		storedPort := forwardedPort{
			remoteHost:   opts.RemoteHost,
			externalPort: port.ExternalPort,
			protocol:     port.Protocol,
		}

		delete(f.existingForwarding, storedPort)

		if err := f.client.AddPortMappingCtx(
			ctx,
			opts.RemoteHost,
			// External port number to expose to Internet:
			port.ExternalPort,
			// Forward TCP (this could be "UDP" if we wanted that instead).
			port.Protocol,
			// Internal port number on the LAN to forward to.
			// Some routers might not support this being different to the external
			// port number.
			port.InternalPort,
			// Internal address on the LAN we want to forward to.
			f.client.LocalAddr().String(),
			// Enabled:
			true,
			// Informational description for the client requesting the port forwarding.
			opts.ProgramName,
			// How long should the port forward last for in seconds.
			// If you want to keep it open for longer and potentially across router
			// resets, you might want to periodically request before this elapses.
			uint32(opts.LeaseDuration.Seconds()),
		); err != nil {
			extErr := fmt.Errorf("error forwarding port %+v: %w", port, err)

			_, _ = fmt.Fprintln(os.Stderr, extErr.Error())

			errs = errors.Join(errs, extErr)
		}

		f.existingForwarding[storedPort] = struct{}{}

		fmt.Printf("Port forwarding created: internal (%d), external (%d), proto (%s)\n",
			port.InternalPort, port.ExternalPort, port.Protocol)
	}

	if errs != nil {
		return fmt.Errorf("error forwarding ports: %w", errs)
	}

	return nil
}

func (f *Forwarder) StopAllForwarding(ctx context.Context) error {
	fmt.Println("Shutting down existing forwarding")

	deleted := make([]forwardedPort, 0, len(f.existingForwarding))

	var errs error

	for fwd := range f.existingForwarding {
		err := f.client.DeletePortMappingCtx(ctx, fwd.remoteHost, fwd.externalPort, fwd.protocol)

		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to delete port fowarding for port %d\n", fwd.externalPort)

			errs = errors.Join(errs, err)
		}

		deleted = append(deleted, fwd)

		fmt.Printf("Port forwarding for external port %d stopped\n", fwd.externalPort)
	}

	for _, deletedPort := range deleted {
		delete(f.existingForwarding, deletedPort)
	}

	if errs != nil {
		return fmt.Errorf("errors stopping forwarding: %w", errs)
	}

	return nil
}
