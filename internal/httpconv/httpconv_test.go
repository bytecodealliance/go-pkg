package httpconv

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	witTypes "go.bytecodealliance.org/pkg/wit/types"
)

func TestFieldValues(t *testing.T) {
	got := FieldValues([]string{"text/html", "application/json"})
	want := [][]uint8{[]uint8("text/html"), []uint8("application/json")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FieldValues: got %v, want %v", got, want)
	}

	if got := FieldValues(nil); len(got) != 0 {
		t.Errorf("FieldValues(nil): got %v, want empty", got)
	}
}

func TestAddEntries(t *testing.T) {
	entries := []witTypes.Tuple2[string, []uint8]{
		{F0: "content-type", F1: []uint8("text/html")},
		{F0: "set-cookie", F1: []uint8("a=1")},
		{F0: "set-cookie", F1: []uint8("b=2")},
	}

	dest := http.Header{}
	AddEntries(dest, entries)

	want := http.Header{
		"Content-Type": {"text/html"},
		"Set-Cookie":   {"a=1", "b=2"},
	}
	if !reflect.DeepEqual(dest, want) {
		t.Errorf("AddEntries: got %v, want %v", dest, want)
	}
}

// benchmarkEntries builds a realistic set of wasi:http field entries.
func benchmarkEntries(n int) []witTypes.Tuple2[string, []uint8] {
	entries := make([]witTypes.Tuple2[string, []uint8], 0, n)
	for i := range n {
		entries = append(entries, witTypes.Tuple2[string, []uint8]{
			F0: fmt.Sprintf("x-custom-header-%d", i),
			F1: []uint8(fmt.Sprintf("value-%d-abcdefghijklmnopqrstuvwxyz", i)),
		})
	}
	return entries
}

// benchmarkHeader builds a realistic net/http.Header.
func benchmarkHeader(n int) http.Header {
	h := http.Header{}
	for i := range n {
		h.Add(fmt.Sprintf("X-Custom-Header-%d", i), fmt.Sprintf("value-%d-abcdefghijklmnopqrstuvwxyz", i))
	}
	h.Add("Set-Cookie", "a=1")
	h.Add("Set-Cookie", "b=2")
	return h
}

func BenchmarkFieldValues(b *testing.B) {
	vals := []string{"text/html", "application/json", "a=1; Path=/; HttpOnly"}
	for b.Loop() {
		_ = FieldValues(vals)
	}
}

func BenchmarkAddEntries(b *testing.B) {
	for _, size := range []int{4, 16, 64} {
		b.Run(fmt.Sprintf("headers=%d", size), func(b *testing.B) {
			entries := benchmarkEntries(size)
			for b.Loop() {
				dest := http.Header{}
				AddEntries(dest, entries)
			}
		})
	}
}

func BenchmarkHeaderToFieldValues(b *testing.B) {
	for _, size := range []int{4, 16, 64} {
		b.Run(fmt.Sprintf("headers=%d", size), func(b *testing.B) {
			h := benchmarkHeader(size)
			for b.Loop() {
				for _, vals := range h {
					_ = FieldValues(vals)
				}
			}
		})
	}
}
