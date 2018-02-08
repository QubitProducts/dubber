package dubber

import (
	"bytes"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestZoneDedupe(t *testing.T) {
	var z1 = `
; comment is ignored
thing.example.com. 10 AAAA 2001:4860:4860::8888
thing2.example.com. 10 IN A 8.8.8.8 ; comment=1 aws.Route53.alias=mything
thing2.example.com. 10 IN A 8.8.8.8
thing.example.com. 10 IN A 8.8.8.8 ; comment=1
thing.example.com. 10 IN A 8.8.8.8 ; comment=1
thing.example.com. 10 IN A 8.8.8.8 ; comment=2
thing.example.com. 10 IN A 9.9.9.9 ; comment=3
`

	var z2 = `thing.example.com.	10	IN	A	8.8.8.8 ; comment=1
thing.example.com.	10	IN	A	8.8.8.8 ; comment=2
thing.example.com.	10	IN	A	9.9.9.9 ; comment=3
thing.example.com.	10	IN	AAAA	2001:4860:4860::8888
thing2.example.com.	10	IN	A	8.8.8.8
thing2.example.com.	10	IN	A	8.8.8.8 ; aws.Route53.alias=mything comment=1`

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

func TestZonePartition(t *testing.T) {
	var zstr = `www.example.com.	10	IN	A	8.8.8.8 ; comment=1
www.example.com.	10	IN	A	8.8.8.8 ; comment=2
www.thing.example.com.	10	IN	A	9.9.9.9 ; comment=3
www2.thing.example.com.	10	IN	AAAA	2001:4860:4860::8888
other.com.	10	IN	A	8.8.8.8
thing2.example2.com.	10	IN	A	8.8.8.8
www.thing2.example2.com.	10	IN	A	8.8.8.8 ; comment=1
`

	var exp = map[string]string{
		"thing.example.com.": `www.thing.example.com.	10	IN	A	9.9.9.9 ; comment=3
www2.thing.example.com.	10	IN	AAAA	2001:4860:4860::8888`,
		"example2.com.": `thing2.example2.com.	10	IN	A	8.8.8.8
www.thing2.example2.com.	10	IN	A	8.8.8.8 ; comment=1`,
		"example.com.": `www.example.com.	10	IN	A	8.8.8.8 ; comment=1
www.example.com.	10	IN	A	8.8.8.8 ; comment=2`,
		"com.": `other.com.	10	IN	A	8.8.8.8`,
	}

	domains := []string{"thing.example.com.", "com.", "example.com.", "example2.com."}
	z, err := ParseZoneData(bytes.NewBuffer([]byte(zstr)))
	if err != nil {
		t.Fatalf("could not pass test zone data, err = %v", err)
	}

	zMap := z.Partition(domains)

	rzmstr := map[string]string{}
	for zn, rz := range zMap {
		rzmstr[zn] = rz.String()
	}

	for zn, ez := range exp {
		gz, ok := rzmstr[zn]
		if !ok {
			t.Fatalf("  expected zone %s is not in result", zn)
		}
		if ez != gz {
			t.Fatalf("  expected zone %s:\n  wanted: %#v\n  got: %#v\n", zn, ez, gz)
		}
	}

	for zn := range rzmstr {
		if _, ok := exp[zn]; !ok {
			t.Fatalf("got unexpected zone %s", zn)
		}
	}

	if !reflect.DeepEqual(exp, rzmstr) {
		t.Fatalf("  expected: %#v\n  got: %#v", exp, rzmstr)
	}
}

func TestZoneDiff(t *testing.T) {
	var test = []struct {
		z1, z2, lz, cz, rz string
	}{
		{
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,

			``,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,
			``,
		},
		{
			``,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,

			``,
			``,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,
		},
		{
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,
			``,

			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,
			``,
			``,
		},
		{
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 3.3.3.3`,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,

			``,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 3.3.3.3`,
			`thing.example.com. 10 IN A 2.2.2.2`,
		},
		{
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 2.2.2.2
thing.example.com. 10 IN A 3.3.3.3`,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 3.3.3.3`,

			`thing.example.com. 10 IN A 2.2.2.2`,
			`thing.example.com. 10 IN A 1.1.1.1
thing.example.com. 10 IN A 3.3.3.3`,
			``,
		},
	}

	for i, st := range test {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			z1, err := ParseZoneData(bytes.NewBuffer([]byte(st.z1)))
			if err != nil {
				t.Fatalf("expected no errors while parsing, got errs = %v", err)
			}

			z2, err := ParseZoneData(bytes.NewBuffer([]byte(st.z2)))
			if err != nil {
				t.Fatalf("expected no errors while parsing, got errs = %v", err)
			}

			sort.Sort(ByRR(z1))
			sort.Sort(ByRR(z2))

			lz, cz, rz := z1.Diff(z2)

			glzstr := strings.Replace(lz.String(), "\t", " ", -1)
			if glzstr != st.lz {
				t.Fatalf("got lz: %v\nwanted: %v", glzstr, st.lz)
			}
			gczstr := strings.Replace(cz.String(), "\t", " ", -1)
			if gczstr != st.cz {
				t.Fatalf("got cz: %v\nwanted: %v", gczstr, st.cz)
			}
			grzstr := strings.Replace(rz.String(), "\t", " ", -1)
			if grzstr != st.rz {
				t.Fatalf("got rz: %v\nwanted: %v", grzstr, st.rz)
			}
		})
	}
}
