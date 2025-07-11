package main

import (
	c "ch3fs/pkg/client"
	"go.uber.org/zap"
	"log"
	"os"
	"time"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Creating logger failed")
		return
	}
	host, _ := os.Hostname()
	time.Sleep(30 * time.Second)
	logger.Sugar().Infof("Benchmark client started on: %s", host)

	client := c.NewClient(logger.Sugar())
	client.RunBenchmark()
}
