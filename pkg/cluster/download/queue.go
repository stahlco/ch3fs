package download

import (
	pb "ch3fs/proto"
	"context"
	utils "github.com/linusgith/goutils/pkg/env_utils"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
	"log"
)

type Job struct {
	Ctx    context.Context
	Req    *pb.RecipeDownloadRequest
	Result chan Res
}

type Res struct {
	Resp *pb.RecipeDownloadResponse
	Err  error
}

type Queue struct {
	ch       chan *Job
	consumer int
	worker   *Worker
}

// NewDownloadQueue initializes and returns a new instance of Queue.
// It creates a buffered channel and starts the defined number of workers (defined in .env as "DOWNLOAD_WORKER").
//
// Parameters:
//   - worker: Implementation of the request processing - w.Process will be called in a goroutine
//
// Returns:
//   - *Queue: Pointer to the Queue instance (Not bounded)
func NewDownloadQueue(worker *Worker) *Queue {

	queue := &Queue{
		ch:       make(chan *Job),
		consumer: utils.NoLog().ParseEnvIntDefault("DOWNLOAD_WORKER", 10),
		worker:   worker,
	}

	queue.startWorkers()
	return queue
}

// Enqueue adds a new Job to the queue for processing.
// Queue is not bounded by a static value (e.g. 1000), it's bounded based on available virtual memory.
//
// Parameters:
//   - j: Pointer to a Job (DownloadRequest) to be enqueued.
//
// Returns:
//   - bool : true if the job was successfully enqueued, false otherwise
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

// startWorkers starts n Worker as goroutines based on the predefined value (defined in .env).
// A Worker will execute the Job and write the response back to the caller (gRPC endpoint).
func (q *Queue) startWorkers() {
	for i := 0; i < q.consumer; i++ {
		go func() {
			for job := range q.ch {
				resp, err := q.worker.ProcessDownloadRequest(job.Ctx, job.Req)
				job.Result <- Res{Resp: resp, Err: err}
			}
		}()
	}
}
