package client

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	rps          = 1
	testDuration = 30 * time.Second
	uploadCount  = 10
)

func (c *Client) InitBenchmark() [uploadCount]string {
	var recipeIDs [uploadCount]string
	backoff := 50.0
	for i := 0; i < uploadCount; i++ {
		for {
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			id, err := c.UploadRandomRecipe()
			if err != nil || id == "" {
				backoff = BackoffWithJitter(backoff)
				continue
			}
			recipeIDs[i] = id
			break
		}
	}

	return recipeIDs
}

func (c *Client) RunBenchmark() {

	recipeIDs := c.InitBenchmark()

	time.Sleep(10 * time.Second)

	c.Logger.Infof("Starting benchmark: %d requests/sec for %s", rps, testDuration)

	ticker := time.NewTicker(time.Second / time.Duration(rps))
	defer ticker.Stop()

	var (
		wg                   sync.WaitGroup
		stopTime             = time.Now().Add(testDuration)
		downloadLatencies    []time.Duration
		uploadLatencies      []time.Duration
		downloadLatenciesMu  sync.Mutex
		uploadLatenciesMu    sync.Mutex
		successDownloadCount int64
		failureDownloadCount int64
		successUploadCount   int64
		failureUploadCount   int64
	)

	for time.Now().Before(stopTime) {
		<-ticker.C
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()

			if rand.Float64() < 0.005 {
				id, err := c.UploadRandomRecipe()
				duration := time.Since(start)
				uploadLatenciesMu.Lock()
				uploadLatencies = append(uploadLatencies, duration)
				uploadLatenciesMu.Unlock()

				if err != nil {
					atomic.AddInt64(&failureUploadCount, 1)
				} else {
					atomic.AddInt64(&successUploadCount, 1)
					recipeIDs[rand.Intn(len(recipeIDs))] = id
				}
			} else {
				fileName := recipeIDs[rand.Intn(len(recipeIDs))]
				res, err := c.DownloadRecipe(fileName)
				duration := time.Since(start)
				downloadLatenciesMu.Lock()
				downloadLatencies = append(downloadLatencies, duration)
				downloadLatenciesMu.Unlock()

				if err != nil || res == nil || !res.Success {
					atomic.AddInt64(&failureDownloadCount, 1)
				} else {
					atomic.AddInt64(&successDownloadCount, 1)
				}
			}
		}()
	}

	wg.Wait()

	dAvg, dP99 := calcBenchmarks(&downloadLatenciesMu, downloadLatencies)
	uAvg, uP99 := calcBenchmarks(&uploadLatenciesMu, uploadLatencies)

	printBenchmarks("DOWNLOAD", successDownloadCount, failureDownloadCount, dAvg, dP99, c)
	printBenchmarks("UPLOAD", successUploadCount, failureUploadCount, uAvg, uP99, c)
}
