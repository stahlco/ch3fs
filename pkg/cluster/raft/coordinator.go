package raft

import (
	"ch3fs/pkg/cluster/transport"
	"context"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	utils "github.com/linusgith/goutils/pkg/env_utils"
	"go.uber.org/zap"
	"log"
	"math/rand"
	"net"
	"os"
	"sort"
	"time"
)

type NodeInformation struct {
	Node    *memberlist.Node
	StartUp time.Time
}

// CoordinateBootstrap coordinates the bootstrapping of a Raft cluster using memberlist for peer discovery.
//
// This function performs the following steps:
//   - Waits for a randomized delay to reduce simultaneous bootstrap contention.
//   - Waits until the expected number of peers (InitialClusterSize) is discovered using memberlist.
//   - Determines the oldest node (based on container startup time) as the bootstrap leader.
//   - If the local node is the bootstrap leader, it initializes the Raft cluster using BootstrapCluster.
//   - If not, the node waits until a leader is elected or the configuration becomes available.
//
// Parameters:
//   - ra: a pointer to the Raft instance (used to bootstrap or check the cluster state).
//   - ml: a pointer to the memberlist instance (used to discover and order cluster members).
//
// Returns:
//   - nil on successful cluster bootstrap or detection of an already bootstrapped cluster.
//   - An error if the discovery times out, bootstrapping fails, or configuration retrieval fails.
func CoordinateBootstrap(ra *raft.Raft, ml *memberlist.Memberlist) error {
	logger := zap.L()

	flag.Parse()

	delay := time.Duration(rand.Intn(20))
	time.Sleep(delay * time.Second)

	start := time.Now()

	for {
		if time.Since(start) > *discoveryTimeout {
			logger.Fatal("Cluster is to slow, discovery timeout reached")
		}

		if len(ml.Members()) >= utils.NoLog().ParseEnvIntDefault("CLUSTER_SIZE", 5) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	members := ml.Members()
	if len(members) == 0 {
		return fmt.Errorf("no members found for bootstrap")
	}

	//Sort members by startup time to ensure consistent ordering
	//Oldest member will be a bootstrapper
	nodeInformations, err := getNodeInformation(ml)
	if err != nil {
		return fmt.Errorf("fetching container startup times for bootstrapping failed with error: %v", err)
	}

	leader := nodeInformations[0].Node
	isBootstrapper := leader.Name == ml.LocalNode().Name

	if isBootstrapper {
		logger.Info("Local node is the bootstrap leader, init cluster")

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
		logger.Info("Successfully bootstrapped cluster", zap.Int("size", len(servers)))
	} else {
		logger.Info("Local node is not the bootstrap leader, waiting for cluster init")
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
				logger.Info("Cluster bootstrap completed, leader detected", zap.Any("LeaderID", lid), zap.Any("LeaderAddr", laddr))
				return nil
			}

			future := ra.GetConfiguration()
			if err = future.Error(); err != nil {
				cfg := future.Configuration()
				if len(cfg.Servers) > 0 {
					logger.Info("Cluster config available", zap.Int("cfg server size", len(cfg.Servers)))
					return nil
				}
			}

		}
	}
}

// getNodeInformation retrieves startup timestamps for all Docker containers
// corresponding to the members in the provided memberlist, and returns a
// slice of NodeInformation sorted by startup time (oldest first).
//
// It performs the following steps:
//   - Creates a Docker client over the UNIX socket (/var/run/docker.sock).
//   - Lists all containers (including stopped ones).
//   - Filters containers whose short ID (first 12 chars) matches a member’s Name.
//   - Inspects each matching container to parse its StartedAt timestamp.
//   - Builds a map of containerName → startTime; missing or unparseable times
//     are defaulted to time.Now().
//   - Assembles NodeInformation entries for each memberlist.Node, warning if
//     no timestamp was found.
//   - Sorts the slice by StartUp ascending and returns it.
//
// Parameters:
//   - ml: the memberlist instance whose Members() will be matched against container IDs.
//
// Returns:
//   - A sorted slice of NodeInformation (the oldest container first).
//   - An error if the Docker client cannot be created or container operations fail.
func getNodeInformation(ml *memberlist.Memberlist) ([]NodeInformation, error) {
	logger := zap.L()
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithVersion("1.47"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker cli from docker sock failed with error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("fetching all containers from cli failed with error: %v", err)
	}

	startupTimes := make(map[string]time.Time)

	for _, c := range containers {
		containerName := c.ID[:12]
		if !transport.ListContainsContainerName(ml, containerName) {
			continue
		}
		inspect, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("inspecting container %s failed with err: %v", containerName, err)
		}

		startTime, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		if err != nil {
			logger.Info("Failed to parse startup time for container", zap.String("container_name", containerName), zap.Error(err))
			startupTimes[containerName] = time.Now()
			continue
		}

		startupTimes[containerName] = startTime
	}

	var nodeInformations []NodeInformation

	for _, m := range ml.Members() {
		startTime, exists := startupTimes[m.Name]
		if !exists {
			logger.Warn("No start time found for container", zap.String("container_name", m.Name))
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

// getRaftAddressFromNode constructs the Raft communication address for a given node.
//
// Parameters:
//   - n: the memberlist.Node whose Address() will be used.
//
// Returns:
//   - A string in the form "host:RPort".
func getRaftAddressFromNode(n *memberlist.Node) string {
	host, _, _ := net.SplitHostPort(n.Address())
	return fmt.Sprintf("%s:%d", host, RPort)
}

// GenerateRaftID creates a unique name of the node based on hostname (container id).
//
// Returns:
//   - Hostname (Container-ID) if hostname lookup was successful.
//   - Alternative ID based on UUIDs.
func GenerateRaftID() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return fmt.Sprintf("replica-%v", uuid.New())
}
