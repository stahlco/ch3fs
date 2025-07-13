package upload

import (
	pb "ch3fs/proto"
	"context"
	utils "github.com/linusgith/goutils/pkg/env_utils"
	mem "github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
	"log"
	"time"
)

type Job struct {
	Ctx    context.Context
	Req    *pb.RecipeUploadRequest
	Result chan Res
}

type Res struct {
	Resp *pb.UploadResponse
	Err  error
}

type Queue struct {
	ch       chan *Job
	consumer int
	worker   *Worker
}

func NewUploadQueue(w *Worker) *Queue {
	queue := &Queue{
		ch:       make(chan *Job),
		consumer: utils.NoLog().ParseEnvIntDefault("UPLOAD_WORKER", 1),
		worker:   w,
	}

	queue.startWorkers()
	return queue
}

// Enqueue adds a job to the upload queue if system memory usage is acceptable.
// If virtual memory usage exceeds 90%, the job is rejected.
//
// Returns true if the job was accepted into the queue; false otherwise.
func (q *Queue) Enqueue(j *Job) bool {
	log.Printf("Now Enqueueing Job: %v", j.Req.Filename)
	logger := zap.S()
	vmem, err := mem.VirtualMemory()
	if err != nil {
		logger.Errorf("Error while fetching current memory data, reject enqueing: %v", err)
		return false
	}
	if vmem.UsedPercent > 90 {
		logger.Warnf("Low Availability of Virtual Memory, rejecting enqueing")
		return false
	}
	q.ch <- j
	return true
}

// startWorkers launches a set of background goroutines based on the
// configured consumer count. Each worker listens to the queue and
// processes incoming upload jobs by calling the associated Worker's
// ProcessUploadRequest method.
func (q *Queue) startWorkers() {
	for i := 0; i < q.consumer; i++ {
		go func() {
			for job := range q.ch {
				log.Printf("Now Processing the Request with Consumer %d", i)
				resp, err := q.worker.ProcessUploadRequest(job.Ctx, job.Req)
				job.Result <- Res{Resp: resp, Err: err}
			}
		}()
	}
}

func (q *Queue) EstimatedQueuingTime() time.Duration {
	if q.worker == nil || q.worker.Estimator == nil || q.consumer == 0 {
		return 0
	}

	workerTime := q.worker.Estimator.Get()
	workers := q.consumer

	queueTime := float64(len(q.ch)) / float64(workers)
	return time.Duration(queueTime * float64(workerTime))
}
