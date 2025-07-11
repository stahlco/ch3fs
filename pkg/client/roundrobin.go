package client

import (
	"sync"
)

type RoundRobin struct {
	ips []string
	idx int
	mu  sync.Mutex
}

func NewRoundRobin(ips []string) *RoundRobin {
	return &RoundRobin{
		ips: ips,
		idx: 0,
	}
}

func (rr *RoundRobin) Next() string {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if len(rr.ips) == 0 {
		return ""
	}

	ip := rr.ips[rr.idx]
	rr.idx = (rr.idx + 1) % len(rr.ips)
	return ip + ":8080"
}
