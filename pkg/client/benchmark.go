package client

import (
	utils "github.com/linusgith/goutils/pkg/env_utils"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRPS          = 10
	defaultTestDuration = 60 * time.Second
	defaultUploadJobs   = 10

	CAP              = 10000
	maxConcurrentReq = 200
)

type Spike struct {
	Start time.Time
	End   time.Time
	RPS   int
}

func (c *Client) InitBenchmark() [defaultUploadJobs]string {
	var recipeIDs [defaultUploadJobs]string
	backoff := 50.0
	for i := 0; i < defaultUploadJobs; i++ {
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
	rps := utils.NoLog().ParseEnvIntDefault("RPS", defaultRPS)
	testDuration := defaultTestDuration
	recipeIDs := c.InitBenchmark()

	c.Logger.Infof("Finished Benchmark setup, waiting 10 sec before flooding the cluster")

	time.Sleep(10 * time.Second)

	c.Logger.Infof("Starting benchmark: %d requests/sec for %s", rps, testDuration)

	var (
		wg        sync.WaitGroup
		semaphore = make(chan struct{}, maxConcurrentReq)
		stopTime  = time.Now().Add(testDuration)
		startTime = time.Now()
		spikePlan = generateClientSpikePlan(startTime)
		now       time.Time

		downloadLatencies, uploadLatencies     []time.Duration
		downloadLatenciesMu, uploadLatenciesMu sync.Mutex
		successDownloadCount                   int64
		failureDownloadCount                   int64
		successUploadCount                     int64
		failureUploadCount                     int64
	)

	//SpikePlan consist of an array of spikes, that have partially random starting points over the testing clients
	//They have a slight variance, but are mostly similar
	//The rps is random between [5, 15]
	//The duration can be [1, 3] seconds

	for now = time.Now(); now.Before(stopTime); now = time.Now() {
		currentRPS, isSpike := getCurrentRPS(spikePlan, now)
		if isSpike {
			log.Printf("Spike: %v RPS=%d", now.Format("15:04:05"), currentRPS)
		}

		ticker := time.NewTicker(time.Second / time.Duration(currentRPS))
		timeout := time.After(time.Second)

	loop:
		for {
			select {
			case <-ticker.C:
				semaphore <- struct{}{}
				wg.Add(1)

				go func() {
					defer func() {
						<-semaphore
						wg.Done()
					}()

					start := time.Now()

					if rand.Float64() < 0.005 {

						id, err := c.UploadRandomRecipe()
						duration := time.Since(start)

						uploadLatenciesMu.Lock()
						uploadLatencies = append(uploadLatencies, duration)
						uploadLatenciesMu.Unlock()

						if err != nil || id == "" {
							atomic.AddInt64(&failureUploadCount, 1)
						} else {
							atomic.AddInt64(&successUploadCount, 1)
							recipeIDs[rand.Intn(len(recipeIDs))] = id
						}

					} else {
						fileName := recipeIDs[rand.Intn(len(recipeIDs))]

						res, err := c.DownloadRecipe(fileName)

						if err != nil || res == nil || !res.Success {
							atomic.AddInt64(&failureDownloadCount, 1)
						}

						duration := time.Since(start)
						downloadLatenciesMu.Lock()
						downloadLatencies = append(downloadLatencies, duration)
						downloadLatenciesMu.Unlock()

						atomic.AddInt64(&successDownloadCount, 1)

					}
				}()
			case <-timeout:
				ticker.Stop()
				break loop
			}
		}
	}

	wg.Wait()

	dAvg, dP99 := calcBenchmarks(&downloadLatenciesMu, downloadLatencies)
	uAvg, uP99 := calcBenchmarks(&uploadLatenciesMu, uploadLatencies)

	printBenchmarks("DOWNLOAD", successDownloadCount, failureDownloadCount, dAvg, dP99, c)
	printBenchmarks("UPLOAD", successUploadCount, failureUploadCount, uAvg, uP99, c)

	log.Printf("Benchmark Completed. Total Duration: %s", time.Since(startTime))
}

func getCurrentRPS(spikePlan []Spike, now time.Time) (int, bool) {
	for _, spike := range spikePlan {
		if now.After(spike.Start) && now.Before(spike.End) {
			return spike.RPS, true
		}
	}
	return defaultRPS, false
}

func generateClientSpikePlan(startTime time.Time) []Spike {
	baseSeconds := []int{15, 25, 45, 50}
	var plan []Spike

	for _, base := range baseSeconds {
		offset := rand.Intn(5) - 2 // each client may vary +- 2 seconds off the
		spikeStart := startTime.Add(time.Duration(base+offset) * time.Second)
		duration := time.Duration(rand.Intn(3)+1) * time.Second // durations of spikes are [1-3s]
		spikeEnd := spikeStart.Add(duration)
		maxSpike := utils.NoLog().ParseEnvIntDefault("SPIKE_MAXIMUM", 50)
		rps := rand.Intn(maxSpike) + 50 // rps are random between [5, 15]

		if spikeStart.Before(startTime) || spikeEnd.After(startTime.Add(defaultTestDuration)) {
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

// ShowSharedState proofs that our system handles shared state, and is scaled out
func (c *Client) ShowSharedState() {

	fileName := getFileName()
	content := getContent()

	if c.currentLeader == "" {
		c.currentLeader = c.RoundRobin.Next()
	}

	// Send Request only once...
	res, err := c.UploadRecipe(c.currentLeader, fileName, []byte(content))
	if err != nil {
		c.Logger.Errorf("Upload failed: %v", err)
	}
	if res == nil {
		c.Logger.Warn("Upload returned nil response")
		return
	}
	if res.Success {
		c.Logger.Infof("Successfully uploaded file: %s", fileName)
	}

	newLeader := res.LeaderContainer + ":8080"
	c.Logger.Infof("Retrying with new leader: %s", newLeader)

	res, err = c.UploadRecipe(newLeader, fileName, content)
	if err != nil || res == nil {
		c.Logger.Errorf("Retry to new leader failed, returning: %v", err)
		return
	}

	if res.Success {
		c.Logger.Infof("Successfully uploaded file after retry: %s", fileName)
		c.currentLeader = newLeader
	}

	//Get Request from every Node...
	time.Sleep(2 * time.Second)
	c.Logger.Infof("Now reading recipe from every other Node")

	n, err := discoverServers(c.Logger)

	for i := 0; i < len(n); i++ {
		res, err := c.DownloadRecipe(fileName)
		if err != nil || res == nil {
			c.Logger.Warnf("Download failed, trying on different node")
			continue
		}
		c.Logger.Infof("Download Recipe: %s with Content: %s", res.Filename, res.Content)
	}
}
