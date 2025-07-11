package cluster

import (
	"context"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/shirou/gopsutil/v3/cpu"
	"go.uber.org/zap"
	"math/rand"
	"time"
)

func getCpuPercent() float64 {
	usage, err := cpu.Percent(time.Second, false)
	if err != nil {
		zap.S().Errorf("Error when fetch CPU Percent interval for a second: %v", err)
		return 100
	}

	return usage[0]
}

// Before enqueuing
func isLeader(r *raft.Raft) (bool, string, error) {
	if r.State() == raft.Leader {
		return true, "", nil
	}

	_, leaderId := r.LeaderWithID()
	if leaderId == "" {
		return false, "", fmt.Errorf("no leader available")
	}

	containerName := string(leaderId)

	return false, containerName, nil
}

// PriorityShedding sheds based on request (upload will be shed, downloads not)
// only the Upload-Endpoint will call this function.
func PriorityShedding() bool {
	return getCpuPercent() > 90
}

func ProbabilisticShedding(upload bool) bool {
	if upload {
		return probabilisticUploadShedding()
	} else {
		return probabilisticDownloadShedding()
	}
}

func probabilisticUploadShedding() bool {
	c := getCpuPercent()

	if c > 80 && rand.Intn(4)%4 == 0 {
		return true
	}
	if c > 75 && c < 80 && rand.Intn(10)%10 == 0 {
		return true
	}
	if c > 70 && c < 75 && rand.Intn(20)%20 == 0 {
		return true
	}

	return false
}

func probabilisticDownloadShedding() bool {
	c := getCpuPercent()

	if c > 95 && rand.Intn(4)%4 == 0 {
		return true
	}
	if c > 90 && c < 80 && rand.Intn(10)%10 == 0 {
		return true
	}
	if c > 85 && c < 75 && rand.Intn(20)%20 == 0 {
		return true
	}

	return false
}

// TODO: Load Shedding based on timeout hint
func TimeoutShedding(ctx context.Context, processTime time.Duration) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return false
	}
	return time.Until(deadline) < processTime
}
