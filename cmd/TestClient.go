package main

import (
	pb "ch3fs/proto"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"math/rand"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	port            = ":8080"
	ch3fTarget      = "ch3f" + port
	rps             = 150
	testDuration    = 10 * time.Second // How long to run the benchmark
	uploadCount     = 10
	benchmarkClient = true
)

var successCount int64
var failureCount int64

func main() {
	host, _ := os.Hostname()
	time.Sleep(10 * time.Second)
	log.Printf("[INFO] Benchmark client started on: %s", host)

	recipeNames := getFileNames(host)
	contents := getContents()
	var recipeIDs [uploadCount]string

	// Upload initial recipes
	for i := 0; i < uploadCount; i++ {
		res, id, err := uploadRecipe(ch3fTarget, recipeNames[i], []byte(contents[i]))
		if err != nil || res == nil {
			log.Printf("[ERROR] Upload failed: %v", err)
			continue
		}
		if !res.Success {
			res, id, err = uploadRecipe(res.LeaderContainer+port, recipeNames[i], []byte(contents[i]))
		}
		recipeIDs[i] = id
	}
	time.Sleep(10 * time.Second)

	// Start rate-limited benchmark
	log.Printf("[INFO] Starting download benchmark: %d requests/sec for %s", rps, testDuration)

	ticker := time.NewTicker(time.Second / time.Duration(rps))
	defer ticker.Stop()

	var (
		wg           sync.WaitGroup
		stopTime     = time.Now().Add(testDuration)
		latencies    []time.Duration
		latenciesMu  sync.Mutex
		successCount int64
		failureCount int64
	)

	for time.Now().Before(stopTime) {
		<-ticker.C
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()

			id := recipeIDs[rand.Intn(uploadCount)]
			res, err := downloadRecipe(id)

			duration := time.Since(start)

			// Store latency
			latenciesMu.Lock()
			latencies = append(latencies, duration)
			latenciesMu.Unlock()

			// Track result
			if err != nil || res == nil || !res.Success {
				atomic.AddInt64(&failureCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// Calculate average and 99th percentile
	latenciesMu.Lock()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	average := total / time.Duration(len(latencies))

	percentile99 := latencies[int(float64(len(latencies))*0.99)]
	latenciesMu.Unlock()

	// Log final metrics
	log.Printf("[RESULT] Success: %d | Failures: %d | Total: %d",
		successCount, failureCount, successCount+failureCount)

	log.Printf("[RESULT] Success Rate: %.2f%%",
		float64(successCount)*100/float64(successCount+failureCount))

	log.Printf("[RESULT] Average Latency: %s | 99th Percentile Latency: %s",
		average, percentile99)

}

func uploadRecipe(target, filename string, content []byte) (*pb.UploadResponse, string, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, "", err
	}
	defer conn.Close()

	client := pb.NewFileSystemClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	id := uuid.New().String()
	req := &pb.RecipeUploadRequest{
		Id:       id,
		Filename: filename,
		Content:  content,
	}
	res, err := client.UploadRecipe(ctx, req)
	return res, id, err
}

func downloadRecipe(id string) (*pb.RecipeDownloadResponse, error) {
	conn, err := grpc.NewClient(ch3fTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewFileSystemClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.RecipeDownloadRequest{RecipeId: id}
	return client.DownloadRecipe(ctx, req)
}

func getFileNames(host string) [10]string {
	return [10]string{
		"chicken_wrap_" + host + ".txt",
		"vegan_burger_" + host + ".txt",
		"banana_pancake_" + host + ".txt",
		"beef_tacos_" + host + ".txt",
		"quinoa_salad_" + host + ".txt",
		"avocado_toast_" + host + ".txt",
		"spaghetti_carbonara_" + host + ".txt",
		"berry_smoothie_" + host + ".txt",
		"grilled_cheese_" + host + ".txt",
		"pumpkin_soup_" + host + ".txt",
	}
}

func getContents() [10]string {
	return [10]string{
		"Chicken, wrap, salad, sauce",
		"Plant patty, bun, lettuce, tomato",
		"Banana, oats, egg, milk",
		"Beef, taco shell, salsa, cheese",
		"Quinoa, tomato, cucumber, olive oil",
		"Avocado, toast, chili flakes",
		"Pasta, egg, bacon, cheese",
		"Berries, banana, yogurt, honey",
		"Bread, cheese, butter",
		"Pumpkin, onion, cream, spices",
	}
}
