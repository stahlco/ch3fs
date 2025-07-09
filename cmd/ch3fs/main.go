package main

import (
	"ch3fs/pkg/cluster"
	pb "ch3fs/proto"
	"context"
	"flag"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"net"
	"time"
)

var (
	raftAddr = flag.String("address", ":50051", "TCP host+port for this node")
)

type Server struct {
	Server  *grpc.Server
	Service *cluster.FileServer
}

func main() {
	flag.Parse()
	logger, _ := zap.NewDevelopment()

	ml, err := cluster.DiscoverAndJoinPeers()
	if err != nil {
		log.Fatalf("failed to setup Memberlist: %v", err)
	}

	raftID := cluster.GenerateRaftID()
	ctx := context.Background()

	node, err := cluster.NewRaftWithReplicaDiscovery(ctx, ml, raftID, ":50051")
	if err != nil {
		log.Fatalf("Failed to initialize Raft node: %v", err)
	}

	cache, err := lru.NewARC(128)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	fileServer := cluster.NewFileServer(node, logger, cache)

	grpcServer := grpc.NewServer()
	pb.RegisterFileSystemServer(grpcServer, fileServer)

	log.Printf("gRPC FileServer listening on :8080")
	server := Server{
		Server:  grpcServer,
		Service: fileServer,
	}

	err = server.Start()
	if err != nil {
		logger.Error("Error while starting service with error, aborting", zap.Error(err))
		return
	}

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

func (s *Server) Start() error {
	logger := zap.L()
	lis, err := net.Listen("tcp", cluster.GRPCPort)
	if err != nil {
		logger.Error("Starting server failed", zap.Error(err))
		return err
	}
	logger.Info("Starting server successful")
	return s.Server.Serve(lis)
}
