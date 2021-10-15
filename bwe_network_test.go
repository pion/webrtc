package webrtc

import (
	"context"
	"fmt"

	"github.com/pion/logging"
	"github.com/pion/transport/vnet"
)

type router struct {
	*vnet.Router
	tbf *vnet.TokenBucketFilter
}

type routerConfig struct {
	cidr      string
	staticIPs []string
}

// TODO(mathis): Add parameters for network condition
func createNetwork(ctx context.Context, left, right routerConfig) (*router, *router, *vnet.Router, error) {
	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "0.0.0.0/0",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create WAN router: %w", err)
	}

	leftRouter, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          left.cidr,
		StaticIPs:     left.staticIPs,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
		NATType: &vnet.NATType{
			Mode: vnet.NATModeNAT1To1,
		},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create leftRouter: %w", err)
	}
	var leftNIC vnet.NIC = leftRouter

	//leftNIC, err = vnet.NewLossFilter(leftNIC, 10)
	//if err != nil {
	//	return nil, nil, nil, err
	//}
	//
	//	leftDelay, err := vnet.NewDelayFilter(leftNIC, 10*time.Millisecond)
	//	if err != nil {
	//		return nil, nil, nil, err
	//	}
	//	go leftDelay.Run(ctx)
	//	leftNIC = leftDelay

	// TODO(mathis): replace TBF by more general Traffic Controller which does
	// rate limitting, min delay, jitter, packet loss
	leftTBF, err := vnet.NewTokenBucketFilter(leftNIC, vnet.TBFRate(1*vnet.MBit))
	if err != nil {
		return nil, nil, nil, err
	}
	leftNIC = leftTBF
	if err = wan.AddNet(leftNIC); err != nil {
		return nil, nil, nil, err
	}
	if err = wan.AddChildRouter(leftRouter); err != nil {
		return nil, nil, nil, err
	}

	rightRouter, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          right.cidr,
		StaticIPs:     right.staticIPs,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
		NATType: &vnet.NATType{
			Mode: vnet.NATModeNAT1To1,
		},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create rightRouter: %w", err)
	}
	var rightNIC vnet.NIC = rightRouter

	//rightNIC, err = vnet.NewLossFilter(rightNIC, 10)
	//if err != nil {
	//	return nil, nil, nil, err
	//}

	//rightDelay, err := vnet.NewDelayFilter(rightNIC, 10*time.Millisecond)
	//if err != nil {
	//	return nil, nil, nil, err
	//}
	//go rightDelay.Run(ctx)
	//rightNIC = rightDelay

	// TODO(mathis): replace TBF by more general Traffic Controller which does
	// rate limitting, min delay, jitter, packet loss
	rightTBF, err := vnet.NewTokenBucketFilter(rightNIC, vnet.TBFRate(1*vnet.MBit))
	if err != nil {
		return nil, nil, nil, err
	}
	rightNIC = rightTBF

	if err = wan.AddNet(rightTBF); err != nil {
		return nil, nil, nil, err
	}
	if err = wan.AddChildRouter(rightRouter); err != nil {
		return nil, nil, nil, err
	}

	if err = wan.Start(); err != nil {
		return nil, nil, nil, err
	}

	return &router{
			Router: leftRouter,
			tbf:    leftTBF,
		}, &router{
			Router: rightRouter,
			tbf:    rightTBF,
		},
		wan,
		nil
}
