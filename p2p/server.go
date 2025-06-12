package p2p

import (
	pb "ch3fs/proto"
	"context"
	"fmt"
	"os"
)

type FunctionServer struct {
	pb.FileSystemServer
}

func (s FunctionServer) DummyTest(ctx context.Context, req *pb.DummyTestRequest) (*pb.DummyTestResponse, error) {
	hn, _ := os.Hostname()
	// do smth
	resp := pb.DummyTestResponse{Msg: fmt.Sprintf("%s and I am Server: %s, ", req.GetMsg(), hn)}
	return &resp, nil
}
