package main

import (
	"bytes"
	"encoding/json"
	"log"
	"time"

	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

type reqBody struct {
	Target     string
	Count      int
	Latency    []time.Duration
	AvgLatency time.Duration
	MaxLatency time.Duration
	MinLatency time.Duration
	Timestamp  time.Time `json:"@timestamp"`
}

// sendPingEs sends the ping results to Elasticsearch
func sendPingEs(dest string, results []pingResult) {
	cfg := elasticsearch7.Config{
		Addresses: esServers,
	}
	es7, _ := elasticsearch7.NewClient(cfg)
	body := reqBody{
		Target:    dest,
		Timestamp: time.Now(),
	}
	minLat := time.Duration(1000000000)
	maxLat := time.Duration(0)
	avg := time.Duration(0)
	for _, result := range results {
		if minLat > result.Latency {
			minLat = result.Latency
		}
		if maxLat < result.Latency {
			maxLat = result.Latency
		}
		avg += result.Latency
		body.Latency = append(body.Latency, result.Latency)
	}
	body.AvgLatency = avg / time.Duration(len(results))
	body.MaxLatency = maxLat
	body.MinLatency = minLat
	data, err := json.Marshal(body)
	if err != nil {
		log.Printf("failed to marshal data: %v", err)
	}

	indexName := "ping-" + time.Now().AddDate(0, 0, -int(time.Now().Weekday())).Format("2006-01-02")
	res, err := es7.Index(indexName, bytes.NewReader(data))
	if err != nil {
		log.Printf("failed to make es request: %v", err)
	}
	log.Printf("es response: %v", res)
}
