package phsserver

import (
	"log"

	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
	"strings"
)

type BucketConfig struct {
	Buckets []float64
}

type HandlerFunc func(http.ResponseWriter, *http.Request)

func NewBucketConfig(config string) (*BucketConfig, error) {
	sizes := strings.Split(config, ":")
	buckets := make([]float64, 0)
	for idx, b := range sizes {
		f, err := strconv.ParseFloat(b, 64)
		if err != nil {
			return nil, fmt.Errorf("Cannot parse %q into float.", b)
		}
		buckets = append(buckets, f)

		if idx >= 1 && buckets[idx-1] > buckets[idx] {
			return nil, fmt.Errorf("Buckets out of order, idx(%d) < idx-1",
				idx)
		}

	}
	c := &BucketConfig{
		Buckets: buckets,
	}
	return c, nil
}

type Metrics struct {
	ReqInflight        prometheus.Gauge
	ReqCounter         *prometheus.CounterVec
	ReqDuration        *prometheus.HistogramVec
	ReqDurationBuckets *BucketConfig
	ReqSize            *prometheus.HistogramVec
	ReqSizeBuckets     *BucketConfig
	RespSize           *prometheus.HistogramVec
	RespSizeBuckets    *BucketConfig
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

	if m.ReqDurationBuckets != nil {
		m.ReqDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_server_requests_durations",
				Help:    "requests latencies in seconds",
				Buckets: m.ReqDurationBuckets.Buckets,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqDuration)
	}
	if m.ReqSizeBuckets != nil {
		m.ReqSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_server_request_sizes",
				Help:    "request size in bytes",
				Buckets: m.ReqSizeBuckets.Buckets,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqSize)
	}
	if m.RespSizeBuckets != nil {
		m.RespSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_server_response_sizes",
				Help:    "respone size in bytes",
				Buckets: m.RespSizeBuckets.Buckets,
			},
			[]string{"code", "method", "handler"},
		)
	}
}

func Wrap(h http.Handler, name string, m *Metrics) http.Handler {

	chain := h

	if m.RespSize != nil {

		chain = promhttp.InstrumentHandlerResponseSize(
			m.RespSize.MustCurryWith(prometheus.Labels{"handler": name}),
			h)
	}

	if m.ReqSize != nil {
		chain = promhttp.InstrumentHandlerRequestSize(
			m.ReqSize.MustCurryWith(prometheus.Labels{"handler": name}),
			chain)
	}

	log.Printf("Registering HandleCounter")
	chain = promhttp.InstrumentHandlerCounter(
		m.ReqCounter.MustCurryWith(prometheus.Labels{"handler": name}),
		chain)

	if m.ReqDuration != nil {
		chain = promhttp.InstrumentHandlerDuration(
			m.ReqDuration.MustCurryWith(prometheus.Labels{"handler": "expensive"}),
			chain)
	}
	return chain
}
