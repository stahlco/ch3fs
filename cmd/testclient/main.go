package main

import (
	pb "ch3fs/proto"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	port            = ":8080"
	ch3fTarget      = "ch3f" + port
	rps             = 100
	testDuration    = 120 * time.Second // How long to run the benchmark
	uploadCount     = 10
	benchmarkClient = true
)

type Client struct {
	Client        pb.FileSystemClient
	Logger        *zap.Logger
	currentLeader string
}

func NewClient(logger zap.Logger) (*Client, func() error) {
	conn, err := grpc.NewClient(ch3fTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil
	}

	client := pb.NewFileSystemClient(conn)

	return &Client{
		Client:        client,
		Logger:        &logger,
		currentLeader: ch3fTarget,
	}, conn.Close
}

const CAP = 10000

func main() {
	logger := zap.L()
	host, _ := os.Hostname()
	time.Sleep(30 * time.Second)
	log.Printf("[INFO] Benchmark client started on: %s", host)

	client, cancel := NewClient(*logger)
	defer cancel()

	var recipeIDs [uploadCount]string

	// Upload initial recipes
	backoff := 50.0
	for i := 0; i < uploadCount; i++ {
		for {
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			id, err := client.UploadRandomRecipe()
			if err != nil || id == "" {
				backoff = BackoffWithJitter(backoff)
				continue
			}
			recipeIDs[i] = id
			break
		}
	}

	time.Sleep(10 * time.Second)

	// Start rate-limited benchmark
	log.Printf("[INFO] Starting download benchmark: %d requests/sec for %s", rps, testDuration)

	ticker := time.NewTicker(time.Second / time.Duration(rps))
	defer ticker.Stop()

	var (
		wg                   sync.WaitGroup
		stopTime             = time.Now().Add(testDuration)
		downloadLatencies    []time.Duration
		uploadLatencies      []time.Duration
		downloadLatenciesMu  = &sync.Mutex{}
		uploadLatenciesMu    = &sync.Mutex{}
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

			randVal := rand.Float64()

			//0.5% each requests to upload a file
			if randVal < 0.005 {

				id, err := client.UploadRandomRecipe()

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

				id := recipeIDs[rand.Intn(len(recipeIDs))]
				res, err := client.DownloadRecipe(id)
				duration := time.Since(start)

				// Store latency
				downloadLatenciesMu.Lock()
				downloadLatencies = append(downloadLatencies, duration)
				downloadLatenciesMu.Unlock()

				// Track result
				if err != nil || res == nil || !res.Success {
					atomic.AddInt64(&failureDownloadCount, 1)
				} else {
					atomic.AddInt64(&successDownloadCount, 1)
				}
			}
		}()
	}

	wg.Wait()
	// Calculate benchmarks for download and upload
	downloadAverage, downloadPercentile := calcBenchmarks(downloadLatenciesMu, downloadLatencies)
	uploadAverage, uploadPercentile := calcBenchmarks(uploadLatenciesMu, uploadLatencies)
	// Log final metrics
	log.Println("------ DOWNLOADS ------")
	printBenchmarks(successDownloadCount, failureDownloadCount, downloadAverage, downloadPercentile)
	log.Println("------ UPLOADS ------")
	printBenchmarks(successUploadCount, failureUploadCount, uploadAverage, uploadPercentile)

}

func printBenchmarks(successCount int64, failureCount int64,
	average time.Duration, percentile99 time.Duration) {
	log.Printf("[RESULT] Successful: %d | Failures: %d | Total: %d",
		successCount, failureCount, successCount+failureCount)

	log.Printf("[RESULT] Success Rate: %.2f%%",
		float64(successCount)*100/float64(successCount+failureCount))

	log.Printf("[RESULT] Average Latency: %s | 99th Percentile Latency: %s",
		average, percentile99)
}

func calcBenchmarks(latenciesMu *sync.Mutex, latencies []time.Duration) (time.Duration, time.Duration) {
	latenciesMu.Lock()

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	average := total / time.Duration(len(latencies))

	percentile99 := latencies[int(float64(len(latencies))*0.99)]
	latenciesMu.Unlock()

	return average, percentile99
}

func (client *Client) UploadRandomRecipe() (string, error) {
	fileName := getFileName()
	content := getContent()
	res, id, err := client.UploadRecipe(client.currentLeader, fileName, content)
	if err != nil || res == nil {
		log.Printf("[ERROR] Upload failed: %v", err)
		return "", err
	}
	if !res.Success {
		client.Logger.Info("Directing the request to the leader")
		if res, id, err = client.UploadRecipe(res.LeaderContainer+port, fileName, content); err != nil || res == nil {
			log.Printf("[ERROR] Upload to leader failed: %v", err)
			return "", err
		}
		if res.Success {
			client.currentLeader = res.LeaderContainer + port
		}
	}
	return id, nil
}

func (c *Client) UploadRecipe(target, filename string, content []byte) (*pb.UploadResponse, string, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	id := uuid.New().String()
	req := &pb.RecipeUploadRequest{
		Id:       id,
		Filename: filename,
		Content:  content,
	}
	res, err := c.Client.UploadRecipe(ctx, req)
	return res, id, err
}

func (c *Client) DownloadRecipe(id string) (*pb.RecipeDownloadResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.RecipeDownloadRequest{RecipeId: id}
	return c.Client.DownloadRecipe(ctx, req)
}

var ingredients = []string{
	"chicken", "beef", "tofu", "rice", "avocado", "cheese",
	"salmon", "egg", "quinoa", "beans", "spinach", "mushroom",
}

var meals = []string{
	"wrap", "bowl", "burger", "pasta", "salad", "soup",
	"sandwich", "stew", "taco", "pizza", "curry", "omelette",
}

var instruction = []string{
	"cook", "stir", "bake", "until golden brown",
}

func getFileName() string {
	return ingredients[rand.Intn(len(ingredients))] + "_" + ingredients[rand.Intn(len(ingredients))] +
		meals[rand.Intn(len(meals))]
}

func getContent() []byte {
	words := append(ingredients, instruction...)
	var content []string

	for i := 0; i < 12; i++ {
		content = append(content, words[rand.Intn(len(words))])
	}

	return []byte(strings.Join(content, " "))
}
func BackoffWithJitter(backoff float64) float64 {
	backoff = backoff * 2
	return math.Min(rand.Float64()*backoff, CAP)
}
