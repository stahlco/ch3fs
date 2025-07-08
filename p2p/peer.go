package p2p

import (
	"ch3fs/pkg/storage"
	pb "ch3fs/proto"
	"fmt"
	"github.com/hashicorp/memberlist"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"path/filepath"
)

type Peer struct {
	//Information - Core identity and membership management
	Node *memberlist.Node

	//client *grpc.ClientConn
	Server  *grpc.Server
	Service *FileServer

	Host           string
	GrpcPort       int
	MemberlistPort int

	//Information about the system
	Peers    *memberlist.Memberlist
	RaftNode *RaftNode
}

func NewPeer(list *memberlist.Memberlist, raft *RaftNode) *Peer {
	s := grpc.NewServer()

	// Create unique database
	hostname, _ := os.Hostname()
	path := filepath.Join("/storage", fmt.Sprintf("%s_bbolt.db", hostname))

	service := NewFileServer(storage.NewStore(path, 0600), raft, list)
	pb.RegisterFileSystemServer(s, service)

	// Returns 172.0. ... :7946 but we want 8080
	addr := list.LocalNode().Address()

	// Splits the IP in Host and PORT for us only relevant is the Host
	host, _, _ := net.SplitHostPort(addr)

	return &Peer{
		Node:           list.LocalNode(),
		Server:         s,
		Service:        service,
		Host:           host,
		GrpcPort:       8080,
		MemberlistPort: 7946,
		Peers:          list,
		RaftNode:       raft,
	}
}

func (p *Peer) Start() error {

	grpcAddr := fmt.Sprintf("%s:%d", p.Host, p.GrpcPort)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Not able to listen on Addr: %v", p.Node.Address())
		return err
	}

	log.Printf("Successfully started the Server on Addr: %s", p.Node.Address())

	return p.Server.Serve(lis)
}

func (p *Peer) Stop() {
	if p.Server != nil {
		p.Server.GracefulStop()
	}
}

func ListContains(l *memberlist.Memberlist, node *memberlist.Node) bool {
	for _, member := range l.Members() {
		if member == node {
			return true
		}
	}

	return false
}

func ListContainsContainerName(l *memberlist.Memberlist, container string) bool {
	for _, m := range l.Members() {
		if m.Name == container {
			return true
		}
	}
	return false
}
