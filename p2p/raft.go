package p2p

import (
	pb "ch3fs/proto"
	"ch3fs/storage"
	"context"
	"flag"
	"fmt"
	transport "github.com/Jille/raft-grpc-transport"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	rbolt "github.com/hashicorp/raft-boltdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	RPort              = 50051
	InitialClusterSize = 10
)

var (
	memberlistPort   = flag.Int("memberlist_port", 7946, "Memberlist port")
	discoveryTimeout = flag.Duration("discovery_timeout", 60*time.Second, "How long to wait for replica discovery")
)

type RaftNode struct {
	Raft             *raft.Raft
	TransportManager *transport.Manager
}

func NewRaftWithReplicaDiscorvery(ctx context.Context, ml *memberlist.Memberlist, raftID string, raftAddr string) (*RaftNode, *storage.Store, error) {

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(raftID)

	basePath := filepath.Join("/storage", fmt.Sprintf("%s_raft", raftID))
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, nil, fmt.Errorf("creating basePath dir failed: %v", err)
	}

	log.Printf("[TRACE] basePath for raft databases create as: %s", basePath)

	logDb, err := rbolt.NewBoltStore(filepath.Join(basePath, "logs.db"))
	if err != nil {
		return nil, nil, fmt.Errorf("creating a log store failed with error: %v", err)
	}

	//Stable Store - stores critical metadata like votes,
	stableDb, err := rbolt.NewBoltStore(filepath.Join(basePath, "stable.db"))
	if err != nil {
		return nil, nil, fmt.Errorf("creating a stable store failed with error: %v", err)
	}

	snapStore, err := raft.NewFileSnapshotStore(basePath, 3, os.Stderr)
	if err != nil {
		return nil, nil, fmt.Errorf("creating file snapshot store failed with error: %v", err)
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
		return nil, nil, fmt.Errorf("failed to listen on %s: %v", raftAddr, err)
	}
	go func() {
		log.Printf("gRPC Raft server listening on %s", raftAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC Serve failed: %v", err)
		}
	}()

	// Might change that later
	fsm := fsm{
		store: persistentStore,
	}

	ra, err := raft.NewRaft(c, fsm, logDb, stableDb, snapStore, tm.Transport())
	if err != nil {
		return nil, nil, fmt.Errorf("creating new raft failed with error: %v", err)
	}

	if shouldBootstrap(ra) {
		go func() {
			if err := coordinateBootstrap(ra, ml); err != nil {
				log.Printf("Bootstrap coordination failed: %v", err)
			}
		}()
	} else {
		log.Printf("Existing Raft state found, skipping bootstrap")
	}

	return &RaftNode{
			Raft:             ra,
			TransportManager: tm,
		},
		persistentStore,
		nil
}

type fsm struct {
	store *storage.Store
}

// Apply This function store the accepted log of the majority of the nodes on each fsm
// The data is a byte[], which represents a RecipeUploadRequest
// => we want to sent the RecipeUploadRequest inside the log.data
func (f fsm) Apply(l *raft.Log) interface{} {

	var request pb.RecipeUploadRequest
	err := proto.Unmarshal(l.Data, &request)
	if err != nil {
		log.Printf("Unmarshal of log.Data failed: %v", err)
		return &pb.UploadResponse{
			Success: false,
		}
	}
	id, err := uuid.Parse(request.Id)
	if err != nil {
		log.Printf("Invalid UUID: %v", err)
		return &pb.UploadResponse{
			Success: false,
		}
	}
	recipe := storage.NewRecipe(id, request.GetFilename(), string(request.GetContent()))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = f.store.StoreRecipe(ctx, recipe)
	if err != nil {
		log.Printf("Store Recipe failed in fsm - apply: %v", err)
		return &pb.UploadResponse{
			Success: false,
		}
	}
	log.Printf("Successfully applied recipe: %v", recipe.Filename)
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
		backoff = BackoffWithJitter(backoff)
		time.Sleep(time.Duration(backoff) * time.Millisecond)
	}
}

func shouldBootstrap(raft *raft.Raft) bool {
	stats := raft.Stats()
	log.Printf("[INFO] Raft stats: %v", stats)
	return stats["last_log_index"] == "0" && stats["last_log_term"] == "0"
}

func coordinateBootstrap(ra *raft.Raft, ml *memberlist.Memberlist) error {
	flag.Parse()
	log.Printf("Coordinating bootstrap among replicas")

	delay := time.Duration(rand.Intn(20))
	time.Sleep(delay * time.Second)

	start := time.Now()

	for {
		if time.Since(start) > *discoveryTimeout {
			log.Fatalf("Cluster is to slow")
		}

		if len(ml.Members()) >= InitialClusterSize {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	members := ml.Members()
	if len(members) == 0 {
		return fmt.Errorf("no members found for bootstrap")
	}

	//Sort members by startup time to ensure consistent ordering
	//Oldest member will be bootstrapper
	nodeInformations, err := getNodeInformation(ml)
	if err != nil {
		return fmt.Errorf("fetching container startup times for bootstrapping failed with error: %v", err)
	}

	leader := nodeInformations[0].Node
	isBootstrapper := leader.Name == ml.LocalNode().Name

	if isBootstrapper {
		log.Printf("This node is the bootstrap leader, initializing cluster")

		var servers []raft.Server

		for _, n := range nodeInformations {
			addr := getRaftAddressFromNode(n.Node)

			servers = append(servers, raft.Server{
				Suffrage: 0,
				ID:       raft.ServerID(n.Node.Name),
				Address:  raft.ServerAddress(addr),
			})
		}

		config := raft.Configuration{Servers: servers}

		f := ra.BootstrapCluster(config)
		if err = f.Error(); err != nil {
			return fmt.Errorf("bootstrapping cluster failed with error: %v", err)
		}

		log.Printf("Successfully bootstrapped cluster with %d servers", len(servers))
	} else {
		log.Printf("This node is not the bootstrap leader, waiting for cluster init")
	}

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for cluster bootstrap")
		case <-ticker.C:

			laddr, lid := ra.LeaderWithID()

			if laddr != "" && lid != "" {
				log.Printf("Cluster bootstrap completed, leader detected with LeaderID: %s and LeaderAddr: %s", lid, laddr)
				return nil
			}

			future := ra.GetConfiguration()
			if err = future.Error(); err != nil {
				cfg := future.Configuration()
				if len(cfg.Servers) > 0 {
					log.Printf("Cluster configuration available with %d servers", len(cfg.Servers))
					return nil
				}
			}

		}
	}

	return nil
}

type NodeInformation struct {
	Node    *memberlist.Node
	StartUp time.Time
}

func getNodeInformation(ml *memberlist.Memberlist) ([]NodeInformation, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithVersion("1.47"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker cli from docker sock failed with error: %v", err)
	}

	ctx := context.Background()
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("fetching all containers from cli failed with error: %v", err)
	}

	startupTimes := make(map[string]time.Time)

	for _, c := range containers {
		containerName := c.ID[:12]
		if !ListContainsContainerName(ml, containerName) {
			continue
		}
		inspect, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("inspecting container %s failed with err: %v", containerName, err)
		}

		startTime, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		if err != nil {
			log.Printf("Failed to parse startup time for container %s: %v", containerName, err)
			startupTimes[containerName] = time.Now()
			continue
		}

		startupTimes[containerName] = startTime
	}

	var nodeInformations []NodeInformation

	for _, m := range ml.Members() {
		startTime, exists := startupTimes[m.Name]
		if !exists {
			log.Printf("[WARN] No start time found for container %s", m.Name)
			startTime = time.Now()
		}

		nodeInformations = append(nodeInformations, NodeInformation{
			Node:    m,
			StartUp: startTime,
		})
	}

	sort.Slice(nodeInformations, func(i, j int) bool {
		return nodeInformations[i].StartUp.Before(nodeInformations[j].StartUp)
	})

	return nodeInformations, nil
}

// Generate a unique ID for local node (container name or random num)
func GenerateRaftID() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return fmt.Sprintf("replica-%v", uuid.New())
}

func getRaftAddressFromNode(n *memberlist.Node) string {
	host, _, _ := net.SplitHostPort(n.Address())
	return fmt.Sprintf("%s:%d", host, RPort)
}
