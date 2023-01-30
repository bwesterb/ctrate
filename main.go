package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	url = flag.String("log", "https://ct.googleapis.com/logs/xenon2022",
		"Base URL of log to test")
	batchSize = flag.Int("batch-size", 1024,
		"Number of entries to fetch at once")
	parallellism = flag.Int("parallellism", 1,
		"Number of parallel outstanding requests")

	todo      chan int
	startTime time.Time

	mux     sync.Mutex
	counter = 0
)

func getTreeSize() int {
	resp, err := http.Get(fmt.Sprintf("%s/ct/v1/get-sth", *url))
	if err != nil {
		log.Fatalf("get-sth GET: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		TreeSize int `json:"tree_size"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		log.Fatalf("get-sth parse: %v", err)
	}
	return body.TreeSize
}

func worker() {
	for {
		batch, ok := <-todo
		if !ok {
			return
		}

		start := batch * *batchSize
		end := (batch + 1) * *batchSize

		for start != end {
			n, err := getEntries(start, end)
			if err != nil {
				log.Printf("%d-%d %v", start, end, err)
				time.Sleep(time.Second)
				continue
			}

			start += n

			mux.Lock()
			counter += n
			mux.Unlock()
		}
	}
}

func getEntries(start, end int) (int, error) {
	resp, err := http.Get(fmt.Sprintf("%s/ct/v1/get-entries?start=%d&end=%d",
		*url, start, end))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var body struct {
		Entries []interface{} `json:"entries"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return 0, err
	}
	return len(body.Entries), nil
}

func main() {
	flag.Parse()

	todo = make(chan int, *batchSize)
	treeSize := getTreeSize()

	go func() {
		for i := 0; i < treeSize / *batchSize; i++ {
			todo <- i
		}
		close(todo)
	}()

	for i := 0; i < *parallellism; i++ {
		go worker()
	}

	total := 0
	val := 0
	startTime = time.Now()
	for {
		mux.Lock()
		val = counter
		counter = 0
		mux.Unlock()

		total += val
		fmt.Printf("%d %f\n", val, float64(total)/time.Since(startTime).Seconds())
		time.Sleep(time.Second)
	}
}
