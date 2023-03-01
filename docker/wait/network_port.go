package wait

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/docker/go-connections/nat"
)

// Implement interface
var _ Strategy = (*NetworkPortStrategy)(nil)

type NetworkPortStrategy struct {
	Port nat.Port
	// all WaitStrategies should have a startupTimeout to avoid waiting infinitely
	startupTimeout time.Duration
	pollInterval   time.Duration
	networkAlias   string
}

// NewHostPortStrategy constructs a default host port strategy
func NewNetworkPortStrategy(port nat.Port, networkAlias string) *NetworkPortStrategy {
	return &NetworkPortStrategy{
		Port:           port,
		networkAlias:   networkAlias,
		startupTimeout: defaultStartupTimeout(),
		pollInterval:   defaultPollInterval(),
	}
}

// fluent builders for each property
// since go has neither covariance nor generics, the return type must be the type of the concrete implementation
// this is true for all properties, even the "shared" ones like startupTimeout

// ForListeningPort is a helper similar to those in Wait.java
// https://github.com/testcontainers/testcontainers-java/blob/1d85a3834bd937f80aad3a4cec249c027f31aeb4/core/src/main/java/org/testcontainers/containers/wait/strategy/Wait.java
func ForNetworkPort(port nat.Port, networkAlias string) *NetworkPortStrategy {
	return NewNetworkPortStrategy(port, networkAlias)
}

func (hp *NetworkPortStrategy) WithStartupTimeout(startupTimeout time.Duration) *NetworkPortStrategy {
	hp.startupTimeout = startupTimeout
	return hp
}

// WithPollInterval can be used to override the default polling interval of 100 milliseconds
func (hp *NetworkPortStrategy) WithPollInterval(pollInterval time.Duration) *NetworkPortStrategy {
	hp.pollInterval = pollInterval
	return hp
}

// WaitUntilReady implements Strategy.WaitUntilReady
func (hp *NetworkPortStrategy) WaitUntilReady(ctx context.Context, target StrategyTarget) (err error) {
	// limit context to startupTimeout
	ctx, cancelContext := context.WithTimeout(ctx, hp.startupTimeout)
	defer cancelContext()

	ipAddress := hp.networkAlias
	if err != nil {
		return
	}

	var waitInterval = hp.pollInterval

	port := hp.Port
	var i = 0

	for port == "" {
		i++

		select {
		case <-ctx.Done():
			return fmt.Errorf("%s:%w", ctx.Err(), err)
		case <-time.After(waitInterval):
			port, err = target.MappedPort(ctx, hp.Port)
			if err != nil {
				fmt.Printf("(%d) [%s] %s\n", i, port, err)
			}
		}
	}

	proto := port.Proto()
	portNumber := port.Int()
	portString := strconv.Itoa(portNumber)

	dialer := net.Dialer{}
	address := net.JoinHostPort(ipAddress, portString)
	for {
		conn, err := dialer.DialContext(ctx, proto, address)
		if err != nil {
			if v, ok := err.(*net.OpError); ok {
				if v2, ok := (v.Err).(*os.SyscallError); ok {
					if isConnRefusedErr(v2.Err) {
						time.Sleep(waitInterval)
						continue
					}
				}
			}
			return err
		} else {
			_ = conn.Close()
			break
		}
	}
	return nil
}
