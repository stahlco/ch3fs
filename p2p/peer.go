package p2p

import (
	pb "ch3fs/proto"
	"ch3fs/storage"
	"fmt"
	"github.com/hashicorp/memberlist"
	"google.golang.org/grpc"
	"log"
	"net"
)

type Peer struct {
	//Information - Core identity and membership management
	Node *memberlist.Node

	//client *grpc.ClientConn
	Server  *grpc.Server
	Service *FunctionServer

	Host           string
	GrpcPort       int
	MemberlistPort int

	//Information about the system
	Peers *memberlist.Memberlist

	// Data Storage Components
	Store *storage.Store
}

func NewPeer(list *memberlist.Memberlist, store *storage.Store) *Peer {
	s := grpc.NewServer()
	service := &FunctionServer{}
	pb.RegisterFunctionsServer(s, service)

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
		Store:          store,
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
