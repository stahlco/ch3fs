package main

import (
	"ch3fs/p2p"
	"log"
	"time"
)

func main() {
	list, err := DiscoverAndJoinPeers()
	if err != nil {
		log.Fatalf("Setting up the membership in the cluster failed with Error: %v", err)
	}

	peer := p2p.NewPeer(list)
	go peer.Start()

	// TestDummy: Testing the Functionality of the DummyFunction service!
	//go TestDummy(list)

	//TestRecipeUpload: Testing the Functionality of the RecipeUpload service!
	//go TestUploadRecipe(list)

	//Background routine which prints the memberlist
	go func() {
		for {
			log.Printf("[INFO] Current Memberlist: %v", list.Members()) //might useful for consensus algorithms
			time.Sleep(time.Second * 20)
		}
	}()

	// waits for goroutines to determine
	select {}
}
