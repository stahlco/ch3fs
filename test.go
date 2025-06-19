package main

import (
	"ch3fs/p2p"
	pb "ch3fs/proto"
	"fmt"
	"github.com/hashicorp/memberlist"
	"log"
	"math/rand"
	"net"
	"time"
)

func TestDummy(list *memberlist.Memberlist) {
	for {
		time.Sleep(5 * time.Second)
		log.Printf("Hello I´m %v and I will send a msg now", list.LocalNode().Address())
		var target string
		for {
			n := rand.Intn(len(list.Members()))
			targetNode := list.Members()[n]
			if targetNode.Address() == list.LocalNode().Address() {
				log.Printf("TargetNode-Addr: %s = LocalNode-Addr: %s", targetNode.Address(), list.LocalNode().Address())
				continue
			}
			host, _, _ := net.SplitHostPort(targetNode.Address())

			target = fmt.Sprintf("%s:%d", host, 8080)
			break
		}

		msg := fmt.Sprintf("Hello from %v", list.LocalNode())
		req := pb.DummyTestRequest{Msg: msg}
		p2p.SendDummyRequest(target, &req)
	}
}

func TestUploadRecipe(list *memberlist.Memberlist) {
	for {
		time.Sleep(5 * time.Second)
		log.Printf("Hello I´m %v and I will send a msg now", list.LocalNode().Address())
		var target string
		for {
			n := rand.Intn(len(list.Members()))
			targetNode := list.Members()[n]
			if targetNode.Address() == list.LocalNode().Address() {
				log.Printf("TargetNode-Addr: %s = LocalNode-Addr: %s", targetNode.Address(), list.LocalNode().Address())
				continue
			}
			host, _, _ := net.SplitHostPort(targetNode.Address())

			target = fmt.Sprintf("%s:%d", host, 8080)
			break
		}
		request := p2p.ConstructRecipeUploadRequest()
		uploadRequest, err := p2p.SendRecipeUploadRequest(target, &request)
		if err != nil {
			log.Println("Error uploading request")
		}
		log.Printf("upload succeeded?: %v", uploadRequest.Success)
	}

}
