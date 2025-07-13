package loadshed

import (
	"context"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"go.uber.org/zap"
	"math/rand"
	"time"
)

// getCpuPercent returns the current system-wide CPU usage over 1 second.
// If it fails to retrieve the data, it defaults to 100%, assuming worst-case load.
func getCpuPercent() float64 {
	usage, err := cpu.Percent(time.Second, false)
	if err != nil {
		zap.S().Errorf("Error when fetch CPU Percent interval for a second: %v", err)
		return 100
	}

	return usage[0]
}

// getDiskInformation returns the percentage of disk used for the given path.
// On error, assumes critical disk usage (100%).
func getDiskInformation(path string) float64 {
	ustat, err := disk.Usage(path)
	if err != nil {
		return 100
	}

	return ustat.UsedPercent
}

// IsLeader checks if the current node is the Raft leader.
// If not, it returns the current leader's container name (if available).
func IsLeader(r *raft.Raft) (bool, string, error) {
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

// ProbabilisticShedding applies a probabilistic load shedding policy based on CPU usage.
// Upload and download requests use different thresholds and probabilities.
func ProbabilisticShedding(upload bool) bool {
	if upload {
		return probabilisticUploadShedding()
	} else {
		return probabilisticDownloadShedding()
	}
}

// probabilisticUploadShedding sheds upload requests based on CPU thresholds with varying probability.
func probabilisticUploadShedding() bool {
	cpuLoad := getCpuPercent()
	switch {
	case cpuLoad > 85:
		return rand.Intn(4) == 0
	case cpuLoad > 75:
		return rand.Intn(10) == 0
	case cpuLoad > 70:
		return rand.Intn(20) == 0
	}
	return false
}

// probabilisticDownloadShedding sheds download requests under CPU conditions.
func probabilisticDownloadShedding() bool {
	cpuLoad := getCpuPercent()
	switch {
	case cpuLoad > 95:
		return rand.Intn(4) == 0
	case cpuLoad > 90:
		return rand.Intn(10) == 0
	case cpuLoad > 85:
		return rand.Intn(20) == 0
	}
	return false
}

// TimeoutShedding sheds requests if the time remaining until the context deadline
// is less than the estimated processing time.
func TimeoutShedding(ctx context.Context, processTime time.Duration) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return false
	}
	return time.Until(deadline) < processTime
}
