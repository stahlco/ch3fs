package cluster

import (
	"ch3fs/pkg/storage"
	pb "ch3fs/proto"
	"context"
	"flag"
	"fmt"
	transport "github.com/Jille/raft-grpc-transport"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	rbolt "github.com/hashicorp/raft-boltdb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	RPort              = 50051
	InitialClusterSize = 5
)

var (
	memberlistPort   = flag.Int("memberlist_port", 7946, "Memberlist port")
	discoveryTimeout = flag.Duration("discovery_timeout", 60*time.Second, "How long to wait for replica discovery")
)

type RaftNode struct {
	Raft             *raft.Raft
	TransportManager *transport.Manager
	Store            *storage.Store
	logger           *zap.Logger
}

// NewRaftWithReplicaDiscovery initializes a new Raft consensus node with gRPC transport
// and persistent storage. It also integrates dynamic replica discovery using a memberlist.
//
// This function performs the following steps:
//  1. Sets up Raft configuration with the given `raftID` as LocalID.
//  2. Creates a unique base path on disk for Raft logs, snapshots, and application state.
//  3. Initializes BoltDB-based log and stable stores for Raft.
//  4. Sets up a file-based snapshot store.
//  5. Initializes a persistent store to store application state (e.g., recipes).
//  6. Configures a gRPC server to act as Raft’s transport layer.
//  7. Launches the gRPC server listener.
//  8. Constructs the Raft FSM (finite state machine) from the application store.
//  9. Creates the actual Raft instance with the above components.
//  10. Coordinates cluster bootstrap using `coordinateBootstrap` if the node
//     detects it should bootstrap the cluster.
//
// Parameters:
//   - ctx: Context for potential timeout or cancellation.
//   - ml: Memberlist instance used for peer discovery in the cluster.
//   - raftID: Unique ID of the local Raft server (e.g., hostname or container name).
//   - raftAddr: gRPC network address where this Raft server will listen (e.g., ":12000").
//   - logger: A zap.Logger instance used for structured logging.
//
// Returns:
//   - *RaftNode: A structure containing the initialized Raft instance, transport manager, and persistent store.
//   - error: An error if any part of the Raft or transport setup fails.
func NewRaftWithReplicaDiscovery(
	ctx context.Context,
	ml *memberlist.Memberlist,
	raftID string,
	raftAddr string,
) (*RaftNode, error) {

	logger := zap.L()

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(raftID)

	basePath := filepath.Join("/storage", fmt.Sprintf("%s_raft", raftID))
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("creating basePath dir failed: %v", err)
	}

	logger.Debug("basePath for raft databases created", zap.String("basePath", basePath))

	logDb, err := rbolt.NewBoltStore(filepath.Join(basePath, "logs.db"))
	if err != nil {
		return nil, fmt.Errorf("creating a log store failed with error: %v", err)
	}

	//Stable Store - stores critical metadata like votes,
	stableDb, err := rbolt.NewBoltStore(filepath.Join(basePath, "stable.db"))
	if err != nil {
		return nil, fmt.Errorf("creating a stable store failed with error: %v", err)
	}

	snapStore, err := raft.NewFileSnapshotStore(basePath, 3, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("creating file snapshot store failed with error: %v", err)
	}

	//creating store for the persistent recipes, stored in each raftNode
	path := filepath.Join(basePath, fmt.Sprintf("%s_bbolt1.db", raftID))
	persistentStore := storage.NewStore(path, 0600)

	tm := transport.New(
		raft.ServerAddress(raftAddr),
		// Defines the Transport Credentials that grpc uses (In this case insecure...)
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	)

	grpcServer := grpc.NewServer()
	tm.Register(grpcServer)
	lis, err := net.Listen("tcp", raftAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %v", raftAddr, err)
	}
	go func() {
		logger.Info("gRPC Raft server listening", zap.String("address", raftAddr))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("gRPC Serve failed", zap.Error(err))
		}
	}()

	// Might change that later
	fsm := fsm{
		store:  persistentStore,
		logger: logger,
	}

	ra, err := raft.NewRaft(c, fsm, logDb, stableDb, snapStore, tm.Transport())
	if err != nil {
		return nil, fmt.Errorf("creating new raft failed with error: %v", err)
	}

	if shouldBootstrap(ra) {
		go func() {
			if err := CoordinateBootstrap(ra, ml); err != nil {
				logger.Error("Bootstrap coordination failed", zap.Error(err))
			}
		}()
	} else {
		logger.Info("Existing Raft state found, skipping bootstrap")
	}

	return &RaftNode{
			Raft:             ra,
			TransportManager: tm,
			Store:            persistentStore,
			logger:           logger,
		},
		nil
}

type fsm struct {
	store  *storage.Store
	logger *zap.Logger
}

// Apply This function store the accepted log of the majority of the nodes on each fsm
// The data is a byte[], which represents a RecipeUploadRequest
// => we want to sent the RecipeUploadRequest inside the log.data
func (f fsm) Apply(l *raft.Log) interface{} {

	var request pb.RecipeUploadRequest
	err := proto.Unmarshal(l.Data, &request)
	if err != nil {
		f.logger.Error("Unmarshal of log.Data failed", zap.Error(err))
		return &pb.UploadResponse{
			Success: false,
		}
	}
	id, err := uuid.Parse(request.Id)
	if err != nil {
		f.logger.Error("Invalid UUID", zap.String("id", request.Id), zap.Error(err))
		return &pb.UploadResponse{
			Success: false,
		}
	}
	recipe := storage.NewRecipe(id, request.GetFilename(), string(request.GetContent()))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = f.store.StoreRecipe(ctx, recipe)
	if err != nil {
		f.logger.Error("Store Recipe failed in fsm - apply", zap.Error(err))
		return &pb.UploadResponse{
			Success: false,
		}
	}

	f.logger.Info("Successfully applied recipe", zap.String("filename", recipe.Filename))
	return &pb.UploadResponse{
		Success: true,
	}
}

func (f fsm) Snapshot() (raft.FSMSnapshot, error) {
	//TODO implement me
	panic("implement me")
}

func (f fsm) Restore(snapshot io.ReadCloser) error {
	//TODO implement me
	panic("implement me")
}

// shouldBootstrap determines whether the given Raft instance needs to bootstrap a new cluster.
//
// It checks if the Raft log is empty by verifying that both the "last_log_index"
// and "last_log_term" are equal to "0".
//
// Parameters:
//   - raft: A pointer to a hashicorp/raft.Raft instance.
//
// Returns:
//   - true if the Raft log is empty and bootstrap is needed.
//   - False otherwise.
func shouldBootstrap(raft *raft.Raft) bool {
	stats := raft.Stats()
	return stats["last_log_index"] == "0" && stats["last_log_term"] == "0"
}
