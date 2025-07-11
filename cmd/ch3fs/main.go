package main

import (
	"ch3fs/pkg/cluster"
	pb "ch3fs/proto"
	"context"
	"flag"
	lru "github.com/hashicorp/golang-lru"
	"github.com/shirou/gopsutil/v3/cpu"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
	logger := zap.S()

	ml, err := cluster.DiscoverAndJoinPeers()
	if err != nil {
		logger.Errorf("failed to setup Memberlist: %v", err)
		return
	}

	raftID := cluster.GenerateRaftID()
	ctx := context.Background()

	node, err := cluster.NewRaftWithReplicaDiscovery(ctx, ml, raftID, ":50051")
	if err != nil {
		logger.Errorf("Failed to initialize Raft node: %v", err)
		return
	}

	cache, err := lru.NewARC(128)
	if err != nil {
		logger.Errorf("Failed to create cache: %v", err)
		return
	}

	fileServer := cluster.NewFileServer(node, logger, cache)

	grpcServer := grpc.NewServer()
	pb.RegisterFileSystemServer(grpcServer, fileServer)

	logger.Infof("gRPC FileServer listening on :8080")
	server := Server{
		Server:  grpcServer,
		Service: fileServer,
	}

	err = server.Start()
	if err != nil {
		logger.Errorf("Error while starting service with error: %v aborting", err)
		return
	}

	go func() {
		for {
			logger.Info("Raft Stats:", zap.Any("raft stats", node.Raft.Stats()))
			logger.Info("Current Memberlist", zap.Any("memberlist", ml.Members()))
			leaderAddr, leaderID := node.Raft.LeaderWithID()
			logger.Info("Current Leader", zap.Any("leaderID", leaderID), zap.Any("leader addr", leaderAddr))
			p, err := cpu.Percent(100*time.Millisecond, false)
			if err != nil {
				logger.Warn("Not able to fetch cpu data", zap.Error(err))

			} else {
				logger.Info("Current CPU usage:", zap.Float64("percent:", p[0]))
			}
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
