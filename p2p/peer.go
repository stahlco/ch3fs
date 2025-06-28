package p2p

import (
	pb "ch3fs/proto"
	"ch3fs/storage"
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"io"
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
	Peers *memberlist.Memberlist
	Raft  *raft.Raft
}

type CH3FSM struct {
	storage *storage.Store
}

type Snapshot struct{}

type RaftCommand struct {
	Upload pb.RecipeUploadRequest //Data inside the command e.g. recipe
}

// Broadcasting of UploadRequests among the RaftNodes
// The are only Upload and Download => only Upload is broadcasted
// log.Data = RecipeUploadRequest
func (f *CH3FSM) Apply(log *raft.Log) interface{} {

	var uploadReq pb.RecipeUploadRequest
	err := proto.Unmarshal(log.Data, &uploadReq)
	if err != nil {
		fmt.Errorf("failed to unmarshal raft command")
		return nil
	}
	err := f.storage.StoreRecipe(ctx)

}

func (f *CH3FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &Snapshot{}, nil
}

func (f *CH3FSM) Restore(rc io.ReadCloser) error {
	return nil
}

func (s *Snapshot) Persist(_ raft.SnapshotSink) error {
	return nil
}
func (s *Snapshot) Release() {}

func NewPeer(list *memberlist.Memberlist) *Peer {
	s := grpc.NewServer()

	// Create unique database
	hostname, _ := os.Hostname()
	path := filepath.Join("/storage", fmt.Sprintf("%s_bbolt.db", hostname))

	service := NewFileServer(storage.NewStore(path, 0600))
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
