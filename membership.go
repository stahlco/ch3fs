package main

import (
	"log"
	"math"
	"math/rand"
	"net"
	"time"
)

import "github.com/hashicorp/memberlist"

const CAP = 5000.0

// DiscoverAndJoinPeers creates a new memberlist node using the default LAN config,
// discovers peers via DNS lookup ("ch3f"), and attempts to join them.
//
// It retries on failure with exponential backoff and jitter until successful.
// Returns the initialized memberlist instance or an error on failure.
func DiscoverAndJoinPeers() (*memberlist.Memberlist, error) {
	list, err := memberlist.Create(memberlist.DefaultLANConfig())
	if err != nil {
		log.Fatalf("Creating memberlist with DefaultLANConfig failed with Error: %v", err)
		return nil, err
	}

	log.Printf("Node %s startet with Addr: %s", list.LocalNode(), list.LocalNode().Addr)

	backoff := 50.0

	// Joining the cluster
	for {
		//Docker's Internal DNS Service - returns IPs of all healthy containers in the 'ch3fs' network
		peerIPs, err := net.LookupHost("ch3f")
		if err != nil {
			log.Fatalf("Look up of host in 'ch3fs' failed, with Error: %v", err)
			return nil, err
		}
		if len(peerIPs) > 0 {
			n, err := list.Join(peerIPs)
			if err == nil {
				log.Printf("[INFO] Successfully joint %d nodes", n)
				return list, nil
			}
		}
		backoff = math.Min(backoff*2, CAP)
		jitter := rand.Float64() * float64(rand.Intn(int(backoff*0.1))) * jitterDirection() //-500ms < ]-(backoff * 0,1),backoff * 0,1[ < 500ms
		time.Sleep(time.Millisecond * time.Duration(backoff+jitter))
	}
}

// jitterDirection defines if the jitter value is negative or positive,
// to get a wider distribution of the backoff interval
func jitterDirection() float64 {
	random := rand.Intn(2)
	if random == 0 {
		return -1.0
	} else {
		return 1.0
	}
}
