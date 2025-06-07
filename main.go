package main

import (
	"log"
	"time"
)

import "github.com/hashicorp/memberlist"

func main() {
	list, err := memberlist.Create(memberlist.DefaultLANConfig())
	if err != nil {
		log.Fatalf("Creating memeberlist with DefaultLANConfig failed with Error: %v", err)
	}

	log.Printf("Node %s startet!", list.LocalNode())

	_, err = list.Join([]string{"ch3fs-ch3f-1"})
	if err != nil {
		log.Fatalf("Was not able to join to ch3fs-ch3f-1")
	}

	for {
		log.Printf("%v", list.Members())
		time.Sleep(time.Second * 20)
	}
}
