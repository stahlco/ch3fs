package main

import (
	"ch3fs/p2p"
	pb "ch3fs/proto"
	"context"
	"flag"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"net"
	"time"
)

var (
	raftAddr = flag.String("address", ":50051", "TCP host+port for this node")
)

func main() {
	flag.Parse()
	logger, _ := zap.NewDevelopment()

	ml, err := p2p.DiscoverAndJoinPeers()
	if err != nil {
		log.Fatalf("failed to setup Memberlist: %v", err)
	}

	raftID := p2p.GenerateRaftID()
	ctx := context.Background()

	node, persistentStore, err := p2p.NewRaftWithReplicaDiscovery(ctx, ml, raftID, ":50051", logger)
	if err != nil {
		log.Fatalf("Failed to initialize Raft node: %v", err)
	}

	fileServer := p2p.NewFileServer(persistentStore, node, *logger)

	//starting gRPC server functionality, enables test client to reach a node on port 8080
	lis, err := net.Listen("tcp", ":8080")
	grpcServer := grpc.NewServer()

	pb.RegisterFileSystemServer(grpcServer, fileServer)

	log.Printf("gRPC FileServer listening on :8080")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC Serve failed: %v", err)
	}

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
