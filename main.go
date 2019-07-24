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
	ExpensiveReqs      *prometheus.CounterVec
	ExpensiveDurations *prometheus.SummaryVec
}

func (s *Service) metricsRegister() {
	s.Metrics.ExpensiveReqs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "expensive_requests_total",
			Help: "How many Expensive (long running) requests processed, partitioned by status",
		},
		[]string{"status"},
	)
	prometheus.MustRegister(s.Metrics.ExpensiveReqs)

	s.Metrics.ExpensiveDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "expensive_durations",
			Help:       "Expensive  requests latencies in seconds",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"status"},
	)
	prometheus.MustRegister(s.Metrics.ExpensiveDurations)
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	log.Printf("%v.ServeHttp called", s)

	fail := rand.Intn(100)%10 == 0
	d := 5 + rand.Float64()*5 - 5
	status := http.StatusOK
	timer := prometheus.NewTimer(
		prometheus.ObserverFunc(func(v float64) {
			s.Metrics.ExpensiveDurations.WithLabelValues(
				fmt.Sprintf("%d", status)).Observe(v)
		}))
	log.Printf("timer = %+v\n", timer)
	defer timer.ObserveDuration()

	if fail {
		d = d / 1000
		status = http.StatusInternalServerError
	}
	log.Printf("fail = %v, Sleeping for %f seconds", fail, d)
	time.Sleep(time.Duration(d) * time.Second)

	log.Printf("incrementing s.Metrics.ExpensiveDurations = %v", s.Metrics.ExpensiveDurations)
	s.Metrics.ExpensiveReqs.WithLabelValues("success").Inc()
	w.WriteHeader(status)
	fmt.Fprintf(w, "%f Seconds", d)
	log.Printf("url=\"%s\"\n  remote=\"%s\" duration = %f, status=%d\n",
		r.URL, r.RemoteAddr, d, status)
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
	s := &Service{Name: "Test1"}
	s.metricsRegister()
	http.Handle("/expensive", s)

	fmt.Println("Hello.")
	log.Fatal(http.ListenAndServe(l, nil))
}
