package p2p

import (
	pb "ch3fs/proto"
	"context"
	"fmt"
	"os"
)

type FunctionServer struct {
	pb.FunctionsServer
}

func (s FunctionServer) DummyTest(ctx context.Context, req *pb.DummyReq) (*pb.DummyResp, error) {
	hn, _ := os.Hostname()
	// do smth
	resp := pb.DummyResp{Msg: fmt.Sprintf("%s and I am Server: %s, ", req.GetMsg(), hn)}
	return &resp, nil
}
