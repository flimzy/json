package json

import (
	"io/ioutil"
	"testing"
)

var largeSlice = make([]Optionals, 1<<10)

func BenchmarkBulk(b *testing.B) {
	b.ReportAllocs()
	enc := NewEncoder(ioutil.Discard)
	for n := 0; n < b.N; n++ {
		_ = enc.Encode(largeSlice)
	}
}

func BenchmarkBulkStream(b *testing.B) {
	b.ReportAllocs()
	enc := NewEncoder(ioutil.Discard)
	enc.SetDirectWrite(true)
	for n := 0; n < b.N; n++ {
		_ = enc.Encode(largeSlice)
	}
}
