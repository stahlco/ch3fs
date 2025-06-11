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
	Server         *grpc.Server
	Service        *FunctionServer
	GrpcPort       int
	MemberlistPort int

	//Information about the system
	Peers *memberlist.Memberlist

	// Data Storage Components
	Store *storage.Store
}

func NewPeer(list *memberlist.Memberlist, store *storage.Store, grpcPort int) *Peer {
	s := grpc.NewServer()
	service := &FunctionServer{}
	pb.RegisterFunctionsServer(s, service)

	return &Peer{
		Node:           list.LocalNode(),
		Server:         s,
		Service:        service,
		GrpcPort:       grpcPort,
		MemberlistPort: 7946,
		Peers:          list,
		Store:          store,
	}
}

func (p *Peer) Start() error {
	addr := p.Node.Address()
	host, _, err := net.SplitHostPort(addr)

	grpcAddr := fmt.Sprintf("%s:%d", host, p.GrpcPort)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Not able to listen on Addr: %v", addr)
		return err
	}

	log.Printf("Successfully started the Server on Addr: %s", addr)

	return p.Server.Serve(lis)
}

func (p *Peer) Stop() {
	if p.Server != nil {
		p.Server.GracefulStop()
	}
}
