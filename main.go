package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"

	"git.bofh.at/mla/phs/pkg/phsserver"
	"git.bofh.at/mla/phs/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Service struct {
	Name    string
	Metrics phsserver.Metrics
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
	m := &phsserver.Metrics{}
	var _x *phsserver.BucketConfig
	_x, err := phsserver.NewBucketConfig("1;2;4;8;16;32;64;128;256;1024")
	if err != nil {
		panic(err)
	}
	m.ReqDurationHBuckets = _x
	m.ReqSizeBuckets = _x
	m.RespSizeBuckets = _x

	_y, err := phsserver.NewPercentileConfig("50;90;99")
	m.ReqDurationPBuckets = _y

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
