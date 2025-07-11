package client

import (
	"fmt"
	"go.uber.org/zap"
	"math"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const host = "ch3f"

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
	return ingredients[rand.Intn(len(ingredients))] + "_" +
		ingredients[rand.Intn(len(ingredients))] + meals[rand.Intn(len(meals))]
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
	backoff *= 2
	return math.Min(rand.Float64()*backoff, 10000)
}

func calcBenchmarks(mu *sync.Mutex, latencies []time.Duration) (time.Duration, time.Duration) {
	mu.Lock()
	defer mu.Unlock()

	if len(latencies) == 0 {
		return 0, 0
	}

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	avg := total / time.Duration(len(latencies))

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p99 := latencies[int(float64(len(latencies))*0.99)]
	return avg, p99
}

func printBenchmarks(label string, success, failure int64, avg, p99 time.Duration, c *Client) {
	total := success + failure
	successRate := float64(success) * 100 / float64(total)
	c.Logger.Infof("[%s] Success: %d | Failures: %d | Total: %d", label, success, failure, total)
	c.Logger.Infof("[%s] Success Rate: %.2f%%", label, successRate)
	c.Logger.Infof("[%s] Avg Latency: %s | 99th %%ile Latency: %s", label, avg, p99)
}

func discoverServers(logger *zap.SugaredLogger) ([]string, error) {
	ips, err := net.LookupHost("ch3f")
	if err != nil {
		logger.Errorf("Not able to fetch ips of host: %s with error: %v", host, err)
		return nil, fmt.Errorf("not able to fetch ips from host %s with error: %v", host, err)
	}
	return ips, nil
}
