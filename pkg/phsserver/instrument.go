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

//BucketConfig stores the values for buckets.
//The values must be strict
//monotonic growing. Use NewBuckconfig to create a configuration from a
// semicolon separated string.
type BucketConfig struct {
	Buckets []float64
}

// PercentileConfig stores percentiles to be reported.
type PercentileConfig struct {
	Percentiles map[float64]float64
}

type HandlerFunc func(http.ResponseWriter, *http.Request)

// NewBuckConfig returns a new bucketconfiguration from a string representation
// of the buckets. The string consits of a semicolon separated list of time values,
// which represent the upper bound of the bucket. Leading and trailing semicolons
// are forbiddem, as well as double semicolons, which would result in an invalid
// border. The value of the entroes, when parsed as floats, must e strict
// monotonic growing.
func NewBucketConfig(config string) (*BucketConfig, error) {
	sizes := strings.Split(config, ";")
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

// NewBuckConfig returns a new bucketconfiguration from a string representation
// of the buckets. The string consits of a semicolon separated list of time values,
// which represent the upper bound of the bucket. Leading and trailing semicolons
// are forbiddem, as well as double semicolons, which would result in an invalid
// border. The value of the entroes, when parsed as floats, must e strict
// monotonic growing.
func NewPercentileConfig(config string) (*PercentileConfig, error) {
	percentiles := make(map[float64]float64)
	sizes := strings.Split(config, ";")

	var e float64
	for _,pwitherr := range sizes {
		d := strings.Split(pwitherr, ":")

		p, err := strconv.ParseFloat(d[0], 64)
		if err != nil {
			return nil, fmt.Errorf("Cannot parse percentile %q of %q into float", d[0], pwitherr)
		}
		if len(d) == 1 {
			switch {
			case p < 90: {
				e = p/1000
			}
			case p < 99: {
				e = 0.01
			}
			default: {
				e = 0.001
			}
			}

			percentiles[p/100] = e
			continue
		} else if len(d) == 2 {
			e, err := strconv.ParseFloat(d[1], 64)

			if err != nil {
				return nil, fmt.Errorf("Cannot parse error %q of %q into float", d[1], pwitherr)
			}
			percentiles[p/100] = e/100
		} else {

		}
	}
	c := &PercentileConfig{
		Percentiles: percentiles,
	}
	return c, nil
}

// Metics holds the prometheus metrics for server side metrics.
type Metrics struct {
	ReqInflight        prometheus.Gauge
	ReqCounter         *prometheus.CounterVec

	ReqDurationHisto       *prometheus.HistogramVec
	ReqDurationHBuckets *BucketConfig

	ReqDurationPercentiles *prometheus.SummaryVec
	ReqDurationPBuckets *PercentileConfig

	ReqSize            *prometheus.HistogramVec
	ReqSizeBuckets     *BucketConfig

	RespSize           *prometheus.HistogramVec
	RespSizeBuckets    *BucketConfig
}

// MetricsRegister registers all the metrics with Prometheus. It takes care  not
// to register the buckets, if they have not been configured. The counters are
// registered anyways.
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
			Help: "http server side requests counter",
		},
		[]string{"code", "method", "handler"},
	)

	prometheus.MustRegister(m.ReqCounter)

	if m.ReqDurationHBuckets != nil {
		m.ReqDurationHisto = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_server_requests_durations",
				Help:    "server side requests latencies in seconds",
				Buckets: m.ReqDurationHBuckets.Buckets,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqDurationHisto)
	}

	if m.ReqDurationPBuckets != nil {
		m.ReqDurationPercentiles = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace: "http",
				Subsystem: "server",
				Name:    "requests_duration_percentiles",
				Help:    "server side requests latencies percentiles",
				Objectives: m.ReqDurationPBuckets.Percentiles,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqDurationHisto)
	}

	if m.ReqSizeBuckets != nil {
		m.ReqSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Subsystem: "server",
				Name:    "request_sizes",
				Help:    "server side request size in bytes",
				Buckets: m.ReqSizeBuckets.Buckets,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqSize)
	}
	if m.RespSizeBuckets != nil {
		m.RespSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Subsystem: "server",
				Name:    "response_sizes",
				Help:    "server side respone size in bytes",
				Buckets: m.RespSizeBuckets.Buckets,
			},
			[]string{"code", "method", "handler"},
		)
	}
}

// Wrap encapculates a http.Handler which collects prometheus metrics.
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

	if m.ReqDurationHBuckets != nil {
		chain = promhttp.InstrumentHandlerDuration(
			m.ReqDurationHisto.MustCurryWith(prometheus.Labels{"handler": "expensive"}),
			chain)
	}
	return chain
}
