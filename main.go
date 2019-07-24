package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"

	"git.bofh.at/mla/phs/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Service struct {
	Name    string
	Metrics Metrics
}

type Metrics struct {
	ReqInflight prometheus.Gauge
	ReqCounter  *prometheus.CounterVec
	ReqDuration *prometheus.HistogramVec
	ReqSize     *prometheus.HistogramVec
	RespSize    *prometheus.HistogramVec
}

func metricsRegister(m *Metrics) {
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

func expensive(w http.ResponseWriter, r *http.Request) {

	fail := rand.Intn(100)%10 == 0
	d := 5 + rand.Float64()*5 - 5
	status := http.StatusOK

	if fail {
		d = d / 1000
		status = http.StatusInternalServerError
	}
	log.Printf("fail = %v, Sleeping for %f seconds", fail, d)
	time.Sleep(time.Duration(d) * time.Second)
	w.WriteHeader(status)
	fmt.Fprintf(w, "%f Seconds", d)
	log.Printf("expensive: url=\"%s\"\n  remote=\"%s\" duration = %f, status=%d\n",
		r.URL, r.RemoteAddr, d, status)
}

func cheap(w http.ResponseWriter, r *http.Request) {

	fail := rand.Intn(100)%10 == 0
	status := http.StatusOK

	if fail {
		status = http.StatusInternalServerError
	}
	log.Printf("cheap: fail = %v", fail)
	w.WriteHeader(status)
	fmt.Fprintf(w, "Cheap")
	log.Printf("cheap: url=\"%s\"\n  remote=\"%s\"  status=%d\n",
		r.URL, r.RemoteAddr, status)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Metric endpoint with wrong url %q", r.URL)
	w.WriteHeader(http.StatusNotFound)
}

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	versionFlag := flag.Bool("version", false, "Version")
	flag.Parse()

	if *versionFlag {
		fmt.Println("Build Date:", version.BuildDate)
		fmt.Println("Git Commit:", version.GitCommit)
		fmt.Println("Version:", version.Version)
		fmt.Println("Go Version:", version.GoVersion)
		fmt.Println("OS / Arch:", version.OsArch)
		return
	}
	promMux := http.NewServeMux()
	promMux.Handle("/metrics", promhttp.Handler())
	promMux.HandleFunc("/", notFoundHandler)
	go func() {
		l, err := net.Listen("tcp", ":9201")
		if err != nil {
			log.Printf("Cannot listen on port 9201. err = %v", err)
			panic("Listen error")
		}
		defer l.Close()
		err = http.Serve(l, promMux)
		if err != nil {
			log.Printf("metrics endpoint error. err = %v", err)
			panic("Serving error")
		}
	}()

	if port != nil {
		p := 8080
		port = &p
	}

	l := fmt.Sprintf(":%d", *port)
	m := &Metrics{}
	metricsRegister(m)

	expensiveHandler := http.HandlerFunc(expensive)
	cheapHandler := http.HandlerFunc(cheap)

	expensiveChain := promhttp.InstrumentHandlerInFlight(m.ReqInflight,
		promhttp.InstrumentHandlerDuration(
			m.ReqDuration.MustCurryWith(
				prometheus.Labels{"handler": "expensive"}),
			promhttp.InstrumentHandlerCounter(
				m.ReqCounter.MustCurryWith(
					prometheus.Labels{"handler": "expensive"}),

				promhttp.InstrumentHandlerRequestSize(
					m.ReqSize.MustCurryWith(
						prometheus.Labels{"handler": "expensive"}),

					promhttp.InstrumentHandlerResponseSize(
						m.RespSize.MustCurryWith(
							prometheus.Labels{"handler": "expensive"}),
						expensiveHandler),
				),
			),
		),
	)

	cheapChain := promhttp.InstrumentHandlerInFlight(m.ReqInflight,
		promhttp.InstrumentHandlerDuration(
			m.ReqDuration.MustCurryWith(
				prometheus.Labels{"handler": "cheap"}),
			promhttp.InstrumentHandlerCounter(
				m.ReqCounter.MustCurryWith(
					prometheus.Labels{"handler": "cheap"}),

				promhttp.InstrumentHandlerRequestSize(
					m.ReqSize.MustCurryWith(
						prometheus.Labels{"handler": "cheap"}),

					promhttp.InstrumentHandlerResponseSize(
						m.RespSize.MustCurryWith(
							prometheus.Labels{"handler": "cheap"}),
						cheapHandler),
				),
			),
		),
	)

	http.Handle("/expensive", expensiveChain)
	http.Handle("/cheap", cheapChain)

	fmt.Println("Hello.")
	log.Fatal(http.ListenAndServe(l, nil))
}
