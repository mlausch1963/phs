package _test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"math"
	"github.com/prometheus/client_golang/prometheus"

	"sort"
	"git.bofh.at/mla/phs/pkg/phsserver"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
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

type bucketResult struct {
	e error
	r []float64
}

func TestBucketParser(t *testing.T) {
	tdata := []struct {
		name  string
		input string
		r     bucketResult
	}{
		{
			"single 1",
			"1",
			bucketResult{
				nil,
				[]float64{1.0},
			},
		},
		{
			"multiple integers",
			"1;2;3;4",
			bucketResult{
				nil,
				[]float64{1.0, 2.0, 3.0, 4.0},
			},
		},
		{
			"multiple integers, trailing colon",
			"1;2;3;4;",
			bucketResult{
				fmt.Errorf("Cannot parse \"\" into float."),
				[]float64{1.0, 2.0, 3.0, 4.0},
			},
		},
		{
			"multiple integers, embedded double colon",
			"1;2;;3;4",
			bucketResult{
				fmt.Errorf("Cannot parse \"\" into float."),
				nil,
			},
		},
		{
			"multiple integers, leading colon",
			";1;2;3;4",
			bucketResult{
				fmt.Errorf("Cannot parse \"\" into float."),
				nil,
			},
		},
		{
			"multiple integers, out of order",
			"1;2;4;3",
			bucketResult{
				fmt.Errorf("Buckets out of order, idx(3) < idx-1"),
				[]float64{1.0, 2.0, 3.0, 4.0},
			},
		},
		{
			"multiple floats",
			"1.2;1024.2;4.12345e4",
			bucketResult{
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
		asfloat := []float64(*bc)
		assert.Equal(t, tst.r.r, asfloat, fmt.Sprintf("%s: buckets equal", tst.name))
	}
}

type percentileResult struct {
	e error
	r map[float64]float64
}


func percentilesEqual(
	exp *map[float64]float64,
	real *map[float64]float64,
	e float64) (bool, string) {

	ke := make([]float64, len(*exp))
	kr := make([]float64, len(*real))

	i := 0
	for k := range *exp {
		ke[i] = k
		i++
	}

	i = 0
	for k := range *real {
		kr[i] = k
		i++
	}
	if len(ke) != len(kr) {
		return false, fmt.Sprintf("Percentiles have different length. Expected: %d, real %d",
			len(ke), len(kr))
	}

	sort.Float64s(ke)
	sort.Float64s(kr)

	for i, k := range ke {
		r := kr[i]
		if math.Abs(r - k) > e {
			return false, fmt.Sprintf("Percentiles differn on pos %d. Expected %f, real %f",
			i, k, r)
		}
	}
	return true, ""
}


func TestPercentileParser(t *testing.T) {
	tdata := []struct {
		name  string
		input string
		r     percentileResult
	}{
		{
			"single 50",
			"50",
			percentileResult{
				nil,
				map[float64]float64{0.5:0.05},
			},
		},
		{
			"multiple no-errors",
			"50;98;99.9",
			percentileResult{
				nil,
				map[float64]float64{
					0.5:0.05,
					0.98:0.01,
					0.999:0.0001,
				},
			},
		},
		{
			"multiple integers, no errors",
			"50:1;99:0.2;99.9:0.01",
			percentileResult{
				nil,
				map[float64]float64{
					0.5:0.01,
					0.99:0.002,
					0.999:0.0001,
					},
			},
		},
		{
			"multiple values, embedded semicolon",
			"20;;50",
			percentileResult{
				fmt.Errorf("Cannot parse percentile \"\" of \"\" into float"),
				nil,
			},
		},
		{
			"multiple values, leading colon",
			";30;40",
			percentileResult{
				fmt.Errorf("Cannot parse percentile \"\" of \"\" into float"),
				nil,
			},
		},
	}
	for _, tst := range tdata {
		bc, err := phsserver.NewPercentileConfig(tst.input)
		assert.Equal(t, tst.r.e, err,
			fmt.Sprintf("%s: NewPercentileConfig returns Error ", tst.name))
		if err != nil {
			continue
		}
		asmap := map[float64]float64(*bc)
		success,msg := percentilesEqual(&tst.r.r, &asmap, 0.0001)
		if !success {
			assert.FailNow(t, fmt.Sprintf("%s: %s", tst.name, msg))

		}
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

	rqDurHisto, err := phsserver.NewBucketConfig("0.001;0.010;0.1;0.5;1;2;5;10")
	rqDurPercentiles, err := phsserver.NewPercentileConfig("50;90;99")

	assert.Equal(t, err, nil)
	m := &phsserver.ServerMetrics{
		ReqDurationHistConf: *rqDurHisto,
		ReqDurationPercentileConf: *rqDurPercentiles,
	}
	phsserver.ServerMetricsRegister(m)

	handler := phsserver.WrapHandler(http.HandlerFunc(_p1Handler), "p1", m)

	l := prometheus.Labels{
		"handler": "p1",
		"method":  "get",
		"code":    "200",
	}

	handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK, "pure handler Status")
	c, err := m.ReqCounter.GetMetricWith(l)
	assert.Equal(t, err, nil, "No metric found")

	// need a client metric to get access to the counter value
	xx := &io_prometheus_client.Metric{}
	c.Write(xx)
	assert.Equal(t, 1.0, *xx.Counter.Value, "counter value")
}
