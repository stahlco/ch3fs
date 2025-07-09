package main

import (
	"ch3fs/pkg/cluster"
	pb "ch3fs/proto"
	"context"
	"flag"
<<<<<<< Updated upstream
	lru "github.com/hashicorp/golang-lru"
=======
>>>>>>> Stashed changes
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"net"
<<<<<<< Updated upstream
	"strconv"
=======
>>>>>>> Stashed changes
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

	logger := zap.L()

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

<<<<<<< Updated upstream
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
=======
	fileServer := cluster.NewFileServer(node, logger)

	//starting gRPC server functionality, enables test client to reach a node on port 8080
	lis, err := net.Listen("tcp", ":8080")
	grpcServer := grpc.NewServer()

	pb.RegisterFileSystemServer(grpcServer, fileServer)

	log.Printf("gRPC FileServer listening on :8080")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC Serve failed: %v", err)
>>>>>>> Stashed changes
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
	lis, err := net.Listen("tcp", strconv.Itoa(cluster.GRPCPort))
	if err != nil {
		logger.Error("Starting server failed", zap.Error(err))
		return err
	}
	logger.Info("Starting server successful")
	return s.Server.Serve(lis)
}
