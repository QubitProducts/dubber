package dubber

import (
	"bytes"
	"sort"
	"testing"
)

var z1 = `
thing2.example.com 10 IN A 8.8.8.8 ; comment 1
thing.example.com 10 IN A 8.8.8.8 ; comment 1
thing.example.com 10 IN A 8.8.8.8 ; comment 1
thing.example.com 10 IN A 8.8.8.8 ; comment 2
thing.example.com 10 IN A 9.9.9.9 ; comment 3
`

var z2 = `thing.example.com.	10	IN	A	8.8.8.8 ; comment 1
thing.example.com.	10	IN	A	8.8.8.8 ; comment 2
thing.example.com.	10	IN	A	9.9.9.9 ; comment 3
thing2.example.com.	10	IN	A	8.8.8.8 ; comment 1`

func TestZoneDedupe(t *testing.T) {
	z, err := ParseZoneData(bytes.NewBuffer([]byte(z1)))
	if err != nil {
		t.Fatalf("expected no errors while parsing, got errs = %v", err)
	}
	sort.Sort(ByRR(z))
	sz := Zone(ByRR(z).Dedupe())
	if sz.String() != z2 {
		t.Fatalf("\n  expected: %q\n  got: %q", z2, sz.String())
	}
}
