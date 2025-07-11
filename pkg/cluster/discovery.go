package cluster

import (
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
	"net"
	"time"
)

// DiscoverAndJoinPeers creates a new memberlist instance and attempts to join
// a cluster of peers using Docker's internal DNS.
//
// This function performs the following steps:
//   - Initializes a memberlist instance using DefaultLANConfig.
//   - Uses Docker's internal DNS (`net.LookupHost`) to resolve IPs for service name `ch3f`
//     which should resolve to all healthy containers in the 'ch3fs' Docker network.
//   - Implements a retry loop with exponential backoff and jitter for robustness.
//   - Continues attempting to join the cluster until successful or a fatal DNS error occurs.
//
// Returns:
//   - A pointer to the initialized *memberlist.Memberlist upon successful join.
//   - An error if DNS resolution or memberlist creation fails.
func DiscoverAndJoinPeers() (*memberlist.Memberlist, error) {
	logger := zap.S()

	list, err := memberlist.Create(memberlist.DefaultLANConfig())
	if err != nil {
		logger.Fatal("Creating memberlist with DefaultLANConfig failed", zap.Error(err))
		return nil, err
	}

	logger.Info("Node started: %v, %v", zap.String("local name", list.LocalNode().Name), zap.String("memberlist_addr", list.LocalNode().Address()))
	backoff := 50.0

	// Joining the cluster
	for {
		//Docker's Internal DNS Service - returns IPs of all healthy containers in the 'ch3fs' network
		peerIPs, err := net.LookupHost("ch3f")
		if err != nil {
			logger.Errorf("Look up of host in 'ch3fs' failed %v", err)
			return nil, err
		}
		if len(peerIPs) > 0 {
			n, err := list.Join(peerIPs)
			if err == nil {
				logger.Infof("Successfully joint nodes: %d", n)
				return list, nil
			}
		}
		backoff = BackoffWithJitter(backoff)
		time.Sleep(time.Duration(backoff) * time.Millisecond)
	}
}
