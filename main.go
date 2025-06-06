package main

import (
	"log"
	"time"
)

func main() {
	log.Printf("We are live!")

	for {
		time.Sleep(time.Second * 10)
	}

}
