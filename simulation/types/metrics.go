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

//type EndpointMetric struct {
//	Endpoint    string
//	Method      string
//	TotalReqs   int
//	SuccessRate float64
//	FailureRate float64
//	P50         time.Duration
//	P95         time.Duration
//	P99         time.Duration
//	MinLatency  time.Duration
//	MaxLatency  time.Duration
//	ErrorCounts map[int]int // status code → count
//	Throughput  float64     // req/sec
//}

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
