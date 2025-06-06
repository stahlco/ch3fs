package main

import (
	"log"
	"time"
)

func main() {

	for {
		log.Printf("We are live!")
		time.Sleep(time.Second * 10)
	}

}
