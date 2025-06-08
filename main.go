package main

import (
	"log"
	"math/rand"
	"net"
	"time"
)

import "github.com/hashicorp/memberlist"

func main() {

	// Creating a Local Memberlist with default configuration
	list, err := memberlist.Create(memberlist.DefaultLANConfig())
	if err != nil {
		log.Fatalf("Creating memberlist with DefaultLANConfig failed with Error: %v", err)
	}

	log.Printf("Node %s startet!", list.LocalNode())

	backoff := 50.0

	// Joining the cluster
	for {
		//Docker's Internal DNS Service - returns all healthy containers in the 'Docker' network
		peerIPs, err := net.LookupHost("ch3f") //ch3fs-ch3f-x and x <= n
		if err != nil {
			log.Fatalf("Look up of host in 'ch3fs' failed, with Error: %v", err)
		}
		if len(peerIPs) > 0 {
			n, err := list.Join(peerIPs)
			if err == nil {
				log.Printf("[INFO] Successfully joint %d nodes", n)
				break
			}
		}
		// Random pseudo-random value in between [0,3)
		backoff *= 2
		jitter := rand.Float64() * float64(rand.Intn(3))
		time.Sleep(time.Millisecond * time.Duration(backoff+jitter))
	}

	for {
		log.Printf("[INFO] Current Memberlist: %v", list.Members())
		time.Sleep(time.Second * 20)
	}
}
