package forwarder

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
)

// EnsureForwarded makes sure all registered port forwardings are still running.
func (f *Forwarder) EnsureForwarded(ctx context.Context) error {
	needsRecreation := maps.Clone(f.existingForwarding)

	existingByExtPort := make(map[uint16]forwardedPort, len(f.existingForwarding))

	for fwd := range f.existingForwarding {
		existingByExtPort[fwd.ExternalPort] = fwd
	}

	for idx := uint16(0); idx < math.MaxUint16; idx++ {
		// v1 API has no port listing method
		fwd, err := f.getPortMappingByIndex(ctx, idx)
		if err != nil {
			break // no more ports
		}

		f.logger.DebugContext(ctx, "Found port mapping", "port", fwd)

		existing, ok := existingByExtPort[fwd.ExternalPort]
		if !ok { // created by someone else
			continue
		}

		fwd.leaseDuration = existing.leaseDuration // getPortMappingByIndex returns REMAINING lease duration

		delete(needsRecreation, *fwd) // will delete only full exact matches
	}

	if len(needsRecreation) > 0 {
		f.logger.InfoContext(ctx, "Ports need re-creation", "ports", needsRecreation)
	}

	var errs error

	for fwd := range needsRecreation {
		if err := f.recreateMapping(ctx, fwd); err != nil {
			errs = errors.Join(err)
		}
	}

	if errs != nil {
		return fmt.Errorf("failed to ensure forwardings: %w", errs)
	}

	return nil
}

func (f *Forwarder) getPortMappingByIndex(ctx context.Context, idx uint16) (*forwardedPort, error) {
	remoteHost, extPort, proto, intPort, _, enabled, label, lease, err :=
		f.client.GetGenericPortMappingEntryCtx(ctx, idx)
	if err != nil {
		return nil, fmt.Errorf("error getting port forwarding: %w", err)
	}

	return &forwardedPort{
		remoteHost: remoteHost,
		ForwardedPort: ForwardedPort{
			InternalPort: intPort,
			ExternalPort: extPort,
			Protocol:     proto,
		},
		label:         label,
		enabled:       enabled,
		leaseDuration: lease, // lease shows REMAINING time, so it can't be compared
	}, nil
}
