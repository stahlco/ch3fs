package main

import (
	"log"
	"os"
	"time"
)

func main() {
	hostname, _ := os.Hostname()
	for {
		log.Printf("We are live! And I am %s", hostname)
		time.Sleep(time.Second * 10)
	}
}
