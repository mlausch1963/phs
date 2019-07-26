package tests

import (
	"fmt"
	"git.bofh.at/mla/phs/pkg/phsserver"
	"github.com/stretchr/testify/assert"
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
			"1:2:3:4:4",
			er{
				nil,
				[]float64{1.0, 2.0, 3.0, 4.0, 4.0},
			},
		},
		{
			"multiple integers, trailing colon",
			"1:2:3:4:4:",
			er{
				nil,
				[]float64{1.0, 2.0, 3.0, 4.0, 4.0},
			},
		},
	}
	for _, tst := range tdata {
		bc, err := phsserver.NewBucketConfig(tst.input)
		assert.Equal(t, tst.r.e, err, "NewBucketConfig returns Error ")
		if err != nil {
			continue
		}
		assert.Equal(t, tst.r.r, bc.Buckets, fmt.Sprintf("%s: equal", tst.name))
	}
}
