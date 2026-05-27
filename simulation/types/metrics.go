package types

import "time"

type RequestMetric struct {
	Endpoint   string
	Method     string
	StatusCode int
	Latency    time.Duration
	Message    string
	Timestamp  time.Time
}

type SimReport struct {
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	TotalUsers     int
	TotalRequests  int
	Requests       []RequestMetric
	DBHeavyPaths   []string // just the endpoint paths flagged by the walker
	PeakThroughput float64
}
