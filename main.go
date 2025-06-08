package main

import (
	"log"
	"time"
)

func main() {
	list, err := DiscoverAndJoinPeers()
	if err != nil {
		log.Fatalf("Setting up the membership in the cluster failed with Error: %v", err)
	}

	//Background routine which prints the memberlist
	go func() {
		for {
			log.Printf("[INFO] Current Memberlist: %v", list.Members())
			time.Sleep(time.Second * 20)
		}
	}()

	// waits for goroutines to determine
	select {}
}
