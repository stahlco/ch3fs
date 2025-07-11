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

func NewDownloadQueue(worker *Worker) *Queue {

	queue := &Queue{
		ch:       make(chan *Job),
		consumer: utils.NoLog().ParseEnvIntDefault("DOWNLOAD_WORKER", 10),
		worker:   worker,
	}

	queue.startWorkers()
	return queue
}

func (q *Queue) Len() int {
	return len(q.ch)
}

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
