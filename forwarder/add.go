package forwarder

import (
	"context"
	"errors"
	"fmt"
)

func (f *Forwarder) ForwardPorts(ctx context.Context, opts ForwardOpts) error {
	externalIP, err := f.client.GetExternalIPAddressCtx(ctx)
	if err != nil {
		return err
	}

	f.logger.Info("Recreating forwarding",
		"externalAddress", externalIP,
		"localAddress", f.client.LocalAddr().String(),
	)

	var errs error

	for _, port := range opts.Ports {
		if port == nil {
			continue
		}

		forwardedPort := forwardedPort{
			remoteHost:    opts.RemoteHost,
			ForwardedPort: *port,
			leaseDuration: uint32(opts.LeaseDuration.Seconds()),
			label:         opts.ProgramName,
			enabled:       true,
		}

		delete(f.existingForwarding, forwardedPort)

		if err := f.recreateMapping(ctx, forwardedPort); err != nil {
			errs = errors.Join(err)

			continue
		}

		f.existingForwarding[forwardedPort] = struct{}{}
	}

	if errs != nil {
		return fmt.Errorf("error forwarding ports: %w", errs)
	}

	return nil
}

func (f *Forwarder) recreateMapping(ctx context.Context, fwd forwardedPort) error {
	// Try to clean up first. That's not optimal, but simplifies workflow.
	err := f.client.DeletePortMappingCtx(ctx, fwd.remoteHost, fwd.ExternalPort, fwd.Protocol)
	if err != nil {
		return fmt.Errorf("failed to delete before create: %w", err)
	}

	clientAddr := f.client.LocalAddr().String()

	if err := f.client.AddPortMappingCtx(
		ctx,
		fwd.remoteHost,
		// External port number to expose to Internet:
		fwd.ExternalPort,
		// Forward TCP (this could be "UDP" if we wanted that instead).
		fwd.Protocol,
		// Internal port number on the LAN to forward to.
		// Some routers might not support this being different to the external
		// port number.
		fwd.InternalPort,
		// Internal address on the LAN we want to forward to.
		clientAddr,
		// Enabled:
		true,
		// Informational description for the client requesting the port forwarding.
		fwd.label,
		// How long should the port forward last for in seconds.
		// If you want to keep it open for longer and potentially across router
		// resets, you might want to periodically request before this elapses.
		fwd.leaseDuration,
	); err != nil {
		return fmt.Errorf("error forwarding port %+v: %w", fwd, err)
	}

	f.logger.Info("Port forwarding created",
		"remoteHost", fwd.remoteHost,
		"internalPort", fwd.InternalPort,
		"internalClient", clientAddr,
		"externalPort", fwd.ExternalPort,
		"protocol", fwd.Protocol,
		"enabled", fwd.enabled,
		"lease", fwd.leaseDuration,
	)

	return nil
}
