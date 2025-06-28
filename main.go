package main

import (
	"ch3fs/p2p"
	"context"
	"flag"
	"log"
	"time"
)

var (
	raftAddr = flag.String("address", ":50051", "TCP host+port for this node")
)

func main() {
	flag.Parse()

	ml, err := p2p.DiscoverAndJoinPeers()
	if err != nil {
		log.Fatalf("failed to setup Memberlist: %v", err)
	}

	raftID := p2p.GenerateRaftID()
	ctx := context.Background()

	node, err := p2p.NewRaftWithReplicaDiscorvery(ctx, ml, raftID, ":50051")
	if err != nil {
		log.Fatalf("Failed to initialize Raft node: %v", err)
	}

	// TestDummy: Testing the Functionality of the DummyFunction service!
	//go TestDummy(list)

	//TestRecipeUpload: Testing the Functionality of the RecipeUpload service!
	//go TestUploadRecipe(list)

	//Background routine which prints the memberlist
	go func() {
		for {
			log.Printf("[INFO] Raft Stats: %v", node.Raft.Stats())
			log.Printf("[INFO] Current Memberlist: %v", ml.Members())
			leaderAddr, leaderID := node.Raft.LeaderWithID()
			log.Printf("[INFO] Current Leader: ID=%s, Addr=%s", leaderID, leaderAddr)

			time.Sleep(20 * time.Second)
		}
	}()

	// waits for goroutines to determine
	select {}
}
