package phsserver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	//	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type BucketConfig struct {
	Buckets []float64
}

func NewBucketConfig(config string) (*BucketConfig, error) {
	sizes := strings.Split(config, ":")
	fmt.Printf("sizes: len = %d, content = %+v\n", len(sizes), sizes)
	c := new(BucketConfig)
	buckets := make([]float64, 0)
	for _, b := range sizes {
		f, err := strconv.ParseFloat(b, 64)
		if err != nil {
			return nil, fmt.Errorf("Cannot parse %q into float.", b)
		}
		buckets = append(buckets, f)
	}
	c.Buckets = buckets
	fmt.Printf("real sizes: len = %d, content = %+v\n", len(c.Buckets), sizes)
	return c, nil
}

type Metrics struct {
	ReqInflight prometheus.Gauge
	ReqCounter  *prometheus.CounterVec
	ReqDuration *prometheus.HistogramVec
	ReqSize     *prometheus.HistogramVec
	RespSize    *prometheus.HistogramVec
}

func MetricsRegister(m *Metrics) {
	m.ReqInflight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_server_requests_inflight",
			Help: "A gauge of requests currently being served",
		},
	)
	prometheus.MustRegister(m.ReqInflight)

	m.ReqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_requests_total",
			Help: "http requests counter",
		},
		[]string{"code", "method", "handler"},
	)
	prometheus.MustRegister(m.ReqCounter)

	m.ReqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_requests_durations",
			Help:    "requests latencies in seconds",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"code", "method", "handler"},
	)
	prometheus.MustRegister(m.ReqDuration)

	m.ReqSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_request_sizes",
			Help:    "request size in bytes",
			Buckets: []float64{128, 1024, 512 * 1024, 1024 * 1024, 512 * 1024 * 1024},
		},
		[]string{"code", "method", "handler"},
	)
	prometheus.MustRegister(m.ReqSize)

	m.RespSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_response_sizes",
			Help:    "respone size in bytes",
			Buckets: []float64{128, 1024, 512 * 1024, 1024 * 1024, 512 * 1024 * 1024},
		},
		[]string{"code", "method", "handler"},
	)
}
