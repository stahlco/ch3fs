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
	host, _ := os.Hostname()
	logger.Sugar().Infof("Client %s started", host)
	if err != nil {
		log.Fatalf("Creating logger failed")
		return
	}

	time.Sleep(30 * time.Second)
	logger.Sugar().Infof("Benchmark-Client: %s, starts to send requests", host)

	client := c.NewClient(logger.Sugar())
	client.RunBenchmark()
}
