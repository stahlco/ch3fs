package main

import (
	"ch3fs/p2p"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

import "github.com/hashicorp/memberlist"

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
		backoff = p2p.BackoffWithJitter(backoff)
		time.Sleep(time.Duration(backoff) * time.Millisecond)
	}
}

func FetchSystemMembers(list *memberlist.Memberlist) ([]*memberlist.Node, error) {
	host, _ := os.Hostname()
	if len(list.Members()) <= 0 {
		return nil, fmt.Errorf("memberlist is empty")
	}
	var members []*memberlist.Node
	for _, member := range list.Members() {
		if member.Name == host {
			continue
		}
		members = append(members, member)
	}
	return members, nil
}
