package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"
	"strings"

	"git.bofh.at/mla/phs/pkg/phsserver"
	"github.com/prometheus/client_golang/prometheus"
	"git.bofh.at/mla/phs/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Service struct {
	Name    string
	Metrics phsserver.Metrics
}

type MonitoredClient struct {
	http.Client
}


func NewMonitoredClient(c http.Client,
	counter *prometheus.CounterVec,
	histo *prometheus.HistogramVec,
	endpoint string) *MonitoredClient{
	mc := &MonitoredClient{c}
	fmt.Printf("@@@mla@@@: Monitored client: mc.Tansport = %+v\n",mc.Transport)
	if mc.Transport == nil {
		mc.Transport = http.DefaultTransport
	}
	mc.Transport = promhttp.InstrumentRoundTripperCounter(
		counter.MustCurryWith(prometheus.Labels{
			"action": "unknown",
			"endpoint": endpoint}),
		mc.Transport)
	mc.Transport = promhttp.InstrumentRoundTripperDuration(
		histo.MustCurryWith(prometheus.Labels{
			"action": "unknown",
			"endpoint": endpoint}),
		mc.Transport)
	return mc
}

var clientCounter *prometheus.CounterVec
var clientHisto *prometheus.HistogramVec


func bp(c *MonitoredClient, req *http.Request) (*http.Response, error ) {
	return 	c.Do(req)

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

	c1 := http.Client {
		Timeout: time.Second * 1,
	}

	c := NewMonitoredClient(c1,
		clientCounter,
		clientHisto,
		"cheap")

	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/cheap",
		strings.NewReader(""))
	if err != nil {
		panic("Cannot construct req for metered client")
	}
	rsp, err := bp(c, req)
	if err != nil {
		panic(fmt.Sprintf("Executing req %+v, return error %+v", req, err))
	}
	log.Printf("Client request: rsp = %+v\n", rsp)
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

func runPrometheusEndpoint(mux *http.ServeMux,  listenAddress string) {
	l, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Printf("Cannot listen on port 9201. err = %v", err)
		panic("Listen error")
	}
	defer l.Close()
	err = http.Serve(l, mux)
	if err != nil {
		log.Printf("metrics endpoint error. err = %v", err)
		panic("Serving error")
	}
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
		runPrometheusEndpoint(promMux, ":9201")
	}()

	if port != nil {
		p := 8080
		port = &p
	}

	clientCounter = prometheus.NewCounterVec (
		prometheus.CounterOpts {
			Namespace: "http",
			Subsystem: "client",
			Name: "requests_total",
			Help: "http client side requests counter",
		},
		[]string{"code", "method", "endpoint", "action"},
	)
	prometheus.MustRegister(clientCounter)

	clientHisto = prometheus.NewHistogramVec(
		prometheus.HistogramOpts {
			Namespace: "http",
				Subsystem: "client",
				Name: "requests_duration",
				Help: "Client side http duration histogram",
				Buckets: []float64{1e-3, 2e-3, 4e-3, 8e-3,
					16e-3, 32e-3, 64e-3,
					128e-3, 256e-3, 1024e-3, 2048e-3},
			},
		[]string{"code", "method", "endpoint", "action"})
	prometheus.MustRegister(clientHisto)

	l := fmt.Sprintf(":%d", *port)

	m := phsserver.NewDefaultMetrics()
	phsserver.MetricsRegister(m)

	expensiveHandler := http.HandlerFunc(expensive)
	cheapHandler := http.HandlerFunc(cheap)

	expensiveChain := phsserver.Wrap(expensiveHandler, "XXX:EXPENSIVE", m)
	cheapChain := phsserver.Wrap(cheapHandler, "XXX:CHEAP", m)

	http.Handle("/expensive", expensiveChain)
	http.Handle("/cheap", cheapChain)

	fmt.Println("Hello.")
	log.Fatal(http.ListenAndServe(l, nil))
}
