package main

import (
	"ch3fs/network"
	"ch3fs/storage"
	"github.com/hashicorp/memberlist"
)

type Peer struct {
	//Information - Core identity and membership management
	Node  *memberlist.Node
	Peers *memberlist.Memberlist

	//Network Components
	Server *network.Server
	Client *network.Client
	Store  *storage.Store
}

func NewPeer(list *memberlist.Memberlist, server *network.Server, client *network.Client, store *storage.Store) {

}
