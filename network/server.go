package network

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"net"
)

type Server struct {
	peer     *peer.Peer
	listener net.Listener
	server   *grpc.Server
}

func NewServer() *Server {
	//TODO
	return nil
}

func (s *Server) Start() {

}
