package client

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	rps          = 1
	testDuration = 60 * time.Second
	uploadCount  = 10
)

type Spike struct {
	Start time.Time
	End   time.Time
	RPS   int
}

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

	//SpikePlan consist of an array of spikes, that have partially random starting points over the testing clients
	//They have a slight variance, but are mostly similar
	//The rps is random between [5, 15]
	//The duration can be [1, 3] seconds
	startTime := time.Now()
	spikePlan := generateClientSpikePlan(startTime)

	now := time.Now()
	for now.Before(stopTime) {
		currentRPS, isSpike := getCurrentRPS(spikePlan, now)
		if isSpike {
			c.Logger.Infof("Spike: %v RPS=%d", now.Format("15:04:05"), currentRPS)
		}

		ticker := time.NewTicker(time.Second / time.Duration(currentRPS))
		timeout := time.After(time.Second)

	loop:
		for {
			select {
			case <-ticker.C:
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
			case <-timeout:
				ticker.Stop()
				break loop
			}
		}
		now = time.Now()
	}

	wg.Wait()

	dAvg, dP99 := calcBenchmarks(&downloadLatenciesMu, downloadLatencies)
	uAvg, uP99 := calcBenchmarks(&uploadLatenciesMu, uploadLatencies)

	printBenchmarks("DOWNLOAD", successDownloadCount, failureDownloadCount, dAvg, dP99, c)
	printBenchmarks("UPLOAD", successUploadCount, failureUploadCount, uAvg, uP99, c)
}

func getCurrentRPS(spikePlan []Spike, now time.Time) (int, bool) {
	for _, spike := range spikePlan {
		if now.After(spike.Start) && now.Before(spike.End) {
			return spike.RPS, true
		}
	}
	return rps, false
}

func generateClientSpikePlan(startTime time.Time) []Spike {
	baseSeconds := []int{15, 25, 45, 50}
	var plan []Spike

	for _, base := range baseSeconds {
		offset := rand.Intn(5) - 2 // each client may vary +- 2 seconds off the
		spikeStart := startTime.Add(time.Duration(base+offset) * time.Second)
		duration := time.Duration(rand.Intn(3)+1) * time.Second // durations of spikes are [1-3s]
		spikeEnd := spikeStart.Add(duration)
		rps := rand.Intn(11) + 5 // rps are random between [5, 15]

		if spikeStart.Before(startTime) || spikeEnd.After(startTime.Add(testDuration)) {
			continue
		}

		plan = append(plan, Spike{
			Start: spikeStart,
			End:   spikeEnd,
			RPS:   rps,
		})
	}
	return plan
}
