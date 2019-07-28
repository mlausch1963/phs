package _test

import (
	"fmt"
	"git.bofh.at/mla/phs/pkg/phsserver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func cmpBc(b1 []float64, b2 []float64) (bool, error) {
	if len(b1) != len(b2) {
		return false, fmt.Errorf("length differene %d::%d", len(b1), len(b2))
	}
	for idx, v := range b1 {
		if b2[idx] != v {
			return false, fmt.Errorf("[]float64 mismatch at idx %d (%f, %f)", idx, v, b2[idx])
		}
	}
	return true, nil
}

type er struct {
	e error
	r []float64
}

func TestBucketParser(t *testing.T) {
	tdata := []struct {
		name  string
		input string
		r     er
	}{
		{
			"single 1",
			"1",
			er{
				nil,
				[]float64{1.0},
			},
		},
		{
			"multiple integers",
			"1:2:3:4",
			er{
				nil,
				[]float64{1.0, 2.0, 3.0, 4.0},
			},
		},
		{
			"multiple integers, trailing colon",
			"1:2:3:4:",
			er{
				fmt.Errorf("Cannot parse \"\" into float."),
				[]float64{1.0, 2.0, 3.0, 4.0},
			},
		},
		{
			"multiple integers, embedded double colon",
			"1:2::3:4",
			er{
				fmt.Errorf("Cannot parse \"\" into float."),
				nil,
			},
		},
		{
			"multiple integers, leading colon",
			":1:2:3:4",
			er{
				fmt.Errorf("Cannot parse \"\" into float."),
				nil,
			},
		},
		{
			"multiple integers, out of order",
			"1:2:4:3",
			er{
				fmt.Errorf("Buckets out of order, idx(3) < idx-1"),
				[]float64{1.0, 2.0, 3.0, 4.0},
			},
		},
		{
			"multiple floats",
			"1.2:1024.2:4.12345e4",
			er{
				nil,
				[]float64{1.2, 1024.2, 41234.5},
			},
		},
	}
	for _, tst := range tdata {
		bc, err := phsserver.NewBucketConfig(tst.input)
		assert.Equal(t, tst.r.e, err, fmt.Sprintf("%s: NewBucketConfig returns Error ", tst.name))
		if err != nil {
			continue
		}
		assert.Equal(t, tst.r.r, bc.Buckets, fmt.Sprintf("%s: buckets equal", tst.name))
	}
}

func _p1Handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
}

func TestMetrics(t *testing.T) {
	/*	tdats := []struct {
			name    string
			handler HandlerFunc
		}{
			{
				name:    "dummy",
				handler: nil,
			},
		}
	*/
	req, err := http.NewRequest("GET", "/p1", nil)
	assert.Equal(t, err, nil)
	rr := httptest.NewRecorder()

	rqDur, err := phsserver.NewBucketConfig("0.001:0.010:0.1:0.5:1:2:5:10")
	assert.Equal(t, err, nil)
	m := &phsserver.Metrics{
		ReqDurationBuckets: rqDur,
	}
	phsserver.MetricsRegister(m)

	handler := phsserver.Wrap(http.HandlerFunc(_p1Handler), "p1", m)

	log.Printf("wrapped handler: %+v", handler)

	xx := &io_prometheus_client.Metric{}

	l := prometheus.Labels{
		"handler": "p1",
		"method":  "GET",
		"code":    "200",
	}

	d := prometheus.DefaultRegisterer.(*prometheus.Registry)
	xy, err := d.Gather()
	assert.Equal(t, err, nil, "gather metrics")

	log.Printf("xy = %+v", xy)

	c, err := m.ReqCounter.GetMetricWith(l)
	assert.Equal(t, err, nil, "No metric found")
	fmt.Printf("counter = %+v\n", c)
	c.Write(xx)
	assert.Equal(t, 0.0, *xx.Counter.Value, "counter value")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusOK, "pure handler Status")

	c, err = m.ReqCounter.GetMetricWith(l)
	assert.Equal(t, err, nil, "No metric found")
	fmt.Printf("counter = %+v\n", c)
	c.Write(xx)
	assert.Equal(t, 1.0, *xx.Counter.Value, "counter value")
}
