package phsserver

import (
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
type BucketConfig []float64;


// PercentileConfig stores percentiles to be reported.
type PercentileConfig  map[float64]float64


//type HandlerFunc func(http.ResponseWriter, *http.Request)

// NewBuckConfig returns a new bucketconfiguration from a string representation
// of the buckets. The string consits of a semicolon separated list of time values,
// which represent the upper bound of the bucket. Leading and trailing semicolons
// are forbiddem, as well as double semicolons, which would result in an invalid
// border. The value of the entroes, when parsed as floats, must e strict
// monotonic growing.
func NewBucketConfig(config string) (*BucketConfig, error) {
	sizes := strings.Split(config, ";")
	buckets := make(BucketConfig, 0)
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
	return &buckets, nil
}

// NewBuckConfig returns a new bucketconfiguration from a string representation
// of the buckets. The string consits of a semicolon separated list of time values,
// which represent the upper bound of the bucket. Leading and trailing semicolons
// are forbiddem, as well as double semicolons, which would result in an invalid
// border. The value of the entroes, when parsed as floats, must e strict
// monotonic growing.
func NewPercentileConfig(config string) (*PercentileConfig, error) {

	percentiles := make(PercentileConfig)
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
	return &percentiles, nil
}

// Metics holds the prometheus metrics for server side metrics.
type ServerMetrics struct {
	ReqInflight        prometheus.Gauge
	ReqCounter         *prometheus.CounterVec

	ReqDurationHisto       *prometheus.HistogramVec
	ReqDurationHistConf BucketConfig

	ReqDurationPercentiles *prometheus.SummaryVec
	ReqDurationPercentileConf PercentileConfig

	ReqSize            *prometheus.HistogramVec
	ReqSizeBuckets     BucketConfig

	RespSize           *prometheus.HistogramVec
	RespSizeBuckets    BucketConfig
}

type ClientMetrics struct {
	ReqCounter *prometheus.CounterVec

	ReqDurationHisto       *prometheus.HistogramVec
	ReqDurationHistConf BucketConfig

	ReqDurationPercentiles *prometheus.SummaryVec
	ReqDurationPercentileConf PercentileConfig
}

func NewDefaultServerMetrics() *ServerMetrics {
	m := &ServerMetrics{
		ReqDurationHistConf: []float64{1e-3, 2e-3, 4e-3, 8e-3, 16e-3,
			32e-3, 64e-3, 128e-3, 256e-3, 512e-3, 1024e-3},
		ReqDurationPercentileConf: map[float64]float64{0.5: 0.05, 0.9:0.01, 0.99:0.001},
		ReqSizeBuckets: []float64{256, 512, 1024, 2048, 4096, 1024*1024, 10*1024*1024},
		RespSizeBuckets: []float64{256, 512, 1024, 2048, 4096, 1024*1024, 10*1024*1024},
	}
	return m
}


func NewDefaultClientMetrics() *ClientMetrics {
	m := &ClientMetrics {
		ReqDurationHistConf: []float64{1e-3, 2e-3, 4e-3, 8e-3, 16e-3,
			32e-3, 64e-3, 128e-3, 256e-3, 512e-3},
			ReqDurationPercentileConf:  map[float64]float64{0.5: 0.05, 0.9:0.01, 0.99:0.001},
	}
	return m
}

func NewSlowBuckets() BucketConfig {
	return []float64{1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5}
}


func NewLargeSizes() BucketConfig {
	return []float64{1024, 10 * 1024, 100* 1024, 1024 * 1024,
		5 * 1024 * 1024, 10 * 1024 * 1024}
}

func ClientMetricsRegister(m *ClientMetrics) {

	m.ReqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "http",
			Subsystem: "client",
			Name: "requests_total",
			Help: "http server side requests counter",
		},
		[]string{"code", "method", "endpoint", "action"},
	)
	if len(m.ReqDurationHistConf) > 0 {
	m.ReqDurationHisto = prometheus.NewHistogramVec(
		prometheus.HistogramOpts {
			Namespace: "http",
			Subsystem: "client",
			Name: "requests_duration",
			Help: "Client side http duration histogram",
			Buckets: m.ReqDurationHistConf,
			},
		[]string{"code", "method", "endpoint", "action"})
	}

	if len(m.ReqDurationPercentileConf) > 0 {
		m.ReqDurationPercentiles = prometheus.NewSummaryVec(
			prometheus.SummaryOpts {
				Namespace: "http",
				Subsystem: "client",
				Name: "request_duration_percentile",
				Objectives: m.ReqDurationPercentileConf,
			},
			[]string{"code", "method", "endpoint", "action"})
	}
}


// SerrverMetricsRegister registers all the metrics with Prometheus. It takes care  not
// to register the buckets, if they have not been configured. The counters are
// registered anyways.
func ServerMetricsRegister(m *ServerMetrics) {
	m.ReqInflight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "http",
			Subsystem: "server",
			Name: "requests_inflight",
			Help: "A gauge of requests currently being served",
		},
	)
	prometheus.MustRegister(m.ReqInflight)

	m.ReqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "http",
			Subsystem: "server",
			Name: "requests_total",
			Help: "http server side requests counter",
		},
		[]string{"code", "method", "handler"},
	)

	prometheus.MustRegister(m.ReqCounter)

	if len(m.ReqDurationHistConf) > 0  {
		m.ReqDurationHisto = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Subsystem: "server",
				Name:    "request_duration",
				Help:    "server side requests latencies in seconds",
				Buckets: m.ReqDurationHistConf,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqDurationHisto)
	}

	if len(m.ReqDurationPercentileConf) > 0 {
		m.ReqDurationPercentiles = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace: "http",
				Subsystem: "server",
				Name:    "request_duration_percentile",
				Help:    "server side requests latencies percentiles",
				Objectives: m.ReqDurationPercentileConf,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqDurationPercentiles)
	}

	if len(m.ReqSizeBuckets) > 0 {
		m.ReqSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Subsystem: "server",
				Name:    "request_size",
				Help:    "server side request size in bytes",
				Buckets: m.ReqSizeBuckets,
			},
			[]string{"code", "method", "handler"},
		)
		prometheus.MustRegister(m.ReqSize)
	}
	if len(m.RespSizeBuckets) > 0 {
		m.RespSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Subsystem: "server",
				Name:    "response_size",
				Help:    "server side respone size in bytes",
				Buckets: m.RespSizeBuckets,
			},
			[]string{"code", "method", "handler"},
		)
	}
}

// Wrap encapsulates a http.Handler which collects prometheus metrics.
func WrapHandler(h http.Handler, name string, m *ServerMetrics) http.Handler {
	chain := h

	chain = promhttp.InstrumentHandlerCounter(
		m.ReqCounter.MustCurryWith(prometheus.Labels{"handler": name}),
		chain)

	chain = promhttp.InstrumentHandlerInFlight(
		m.ReqInflight,
		chain)

	if m.RespSize != nil {

		chain = promhttp.InstrumentHandlerResponseSize(
			m.RespSize.MustCurryWith(prometheus.Labels{"handler": name}),
			chain)
	}

	if m.ReqSize != nil {
		chain = promhttp.InstrumentHandlerRequestSize(
			m.ReqSize.MustCurryWith(prometheus.Labels{"handler": name}),
			chain)
	}

	if len(m.ReqDurationHistConf) > 0 {
		chain = promhttp.InstrumentHandlerDuration(
			m.ReqDurationHisto.MustCurryWith(prometheus.Labels{"handler": name}),
			chain)
	}
	if len(m.RespSizeBuckets)  > 0 {
		chain = promhttp.InstrumentHandlerDuration(
			m.ReqDurationPercentiles.MustCurryWith(prometheus.Labels{"handler": name}),
			chain)
	}
	return chain
}
