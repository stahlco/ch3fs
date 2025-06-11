package main

import (
	"ch3fs/p2p"
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
		p2p.SendDummyRequest(target, msg)
	}
}
