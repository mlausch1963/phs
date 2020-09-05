package main

import (
	"flag"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"
	"io/ioutil"

	"git.bofh.at/mla/phs/pkg/phsserver"
	"github.com/prometheus/client_golang/prometheus"
	"git.bofh.at/mla/phs/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/url"
	"bytes"
)

type Service struct {
	Name    string
	ServerMetrics phsserver.ServerMetrics
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


func bp(c *MonitoredClient, req *http.Request) (*http.Response, error ) {
	return 	c.Do(req)

}


type actionIdType int
const (
	actionIdKey actionIdType = iota
)

type Client struct {
	// HTTP client used to communicate with the DO API.
	client *http.Client
	BaseURL *url.URL

	// Prometheus Metrics
	ReqCounter *prometheus.CounterVec
	ReqDurationHisto *prometheus.HistogramVec

	// Add whatever is needed here
	ExternalService *ExternalServiceOp
}

func (c *Client) NewRequest(
	ctx context.Context,
	method, urlStr string,
	body interface{}) (*http.Request, error) {

	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.ResolveReference(rel)
	buf := new(bytes.Buffer)
	s := u.String()

	req, err := http.NewRequest(method, s, buf)
	return req, nil
}

// Wrap encapsulates a http.Handler which collects prometheus metrics.
//func WrapReq(h http.Handler, name string, m *ClientMetrics) http.Handler {
//

func (c *Client) Do(ctx context.Context, enndpointId string,
	req *http.Request, v interface{}) (*http.Response, error) {
	req = req.WithContext(ctx)
	resp, err := c.client.Do(req)
	return resp, err
}

type ExternalService  interface {
	Get(context.Context) (*http.Response, error)
}


type ExternalServiceOp struct {
	client *Client
}


var _ ExternalService = &ExternalServiceOp{}


func (s ExternalServiceOp) Get(ctx context.Context) (*http.Response, error) {
	req, err := s.client.NewRequest(ctx,  http.MethodGet, "http://localhost:5080/cheap", nil)
	resp, err := s.client.Do(ctx, "cheap:get", req, nil)
	return resp, err
}


func NewSvcClient(httpClient *http.Client) *Client {

	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	c := &Client{
		client: httpClient,
	}
	url, err := url.Parse("http://localhost:8080/")
	if err != nil {
		panic("Invalid base url for client")
	}
	c.BaseURL = url
	c.ExternalService = &ExternalServiceOp{client: c}
	return c
}

func expensive(w http.ResponseWriter, r *http.Request) {

	fail := rand.Intn(100)%10 == 0
	d := rand.Float64()*5
	status := http.StatusOK

	if fail {
		d = d / 1000
		status = http.StatusInternalServerError
	}
	log.Printf("fail = %v, Sleeping for %f seconds", fail, d)

	ctx := context.WithValue(context.Background(),
		actionIdKey, "expensive")

	c := NewSvcClient(nil)

	rsp, err := c.ExternalService.Get(ctx)

	if err != nil {
		log.Printf("Request to 'ExternalService.Get' failed. err = %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body, err := ioutil.ReadAll(rsp.Body)
	bodyString := string(body)
	log.Printf("Client returns: %s\n", bodyString)

	time.Sleep(time.Duration(d) * time.Second)

	fmt.Fprintf(w, "%f Seconds", d)
	log.Printf("expensive: url=\"%s\"\n  remote=\"%s\" duration = %f, status=%d\n",
		r.URL, r.RemoteAddr, d, status)


	//w.WriteHeader(status)

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
	port := flag.Int("port", 5080, "Port to listen on")
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
		runPrometheusEndpoint(promMux, ":5201")
	}()

	if port == nil {
		p := 5080
		port = &p
	}


	l := fmt.Sprintf(":%d", *port)

	serverMetric := phsserver.NewDefaultServerMetrics()
	phsserver.ServerMetricsRegister(serverMetric)

	clientMetric := phsserver.NewDefaultClientMetrics()
	phsserver.ClientMetricsRegister(clientMetric)

	expensiveHandler := http.HandlerFunc(expensive)
	cheapHandler := http.HandlerFunc(cheap)

	expensiveMeteredHandler := phsserver.WrapHandler(expensiveHandler, "XXX:EXPENSIVE", serverMetric)
	cheapMeteredHandler := phsserver.WrapHandler(cheapHandler, "XXX:CHEAP", serverMetric)

	http.Handle("/expensive", expensiveMeteredHandler)
	http.Handle("/cheap", cheapMeteredHandler)

	fmt.Println("Hello.")
	log.Fatal(http.ListenAndServe(l, nil))
}
