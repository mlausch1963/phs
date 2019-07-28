package _test

import (
	"fmt"
	"testing"

	"git.bofh.at/mla/phs/pkg/phsserver"
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
