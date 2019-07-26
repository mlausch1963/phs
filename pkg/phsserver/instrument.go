package phsserver

import (
	"github.com/prometheus/client_golang/prometheus"
	//	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
