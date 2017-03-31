// Copyright 2017 Qubit Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dubber

import (
	"io"
	"sort"
	"strings"

	"github.com/miekg/dns"
)

// Record represents a DNS record we wish to be present,
// along with a Flags string which may contain hints to the
// provisioner
type Record struct {
	dns.RR
	Flags string
}

// String implements fmt.Stringer for a Record
func (r *Record) String() string {
	str := r.RR.String()
	if len(r.Flags) != 0 {
		str += " " + r.Flags
	}
	return str
}

// Compare two Records
func (r *Record) Compare(r2 *Record) int {
	hi, hj := r.Header(), r2.Header()
	if c := strings.Compare(hi.Name, hj.Name); c != 0 {
		return c
	}

	if hi.Ttl != hj.Ttl {
		return int(hi.Ttl) - int(hj.Ttl)
	}

	if hi.Class != hj.Class {
		return int(hi.Class) - int(hj.Class)
	}

	if hi.Rrtype != hj.Rrtype {
		return int(hi.Rrtype) - int(hj.Rrtype)
	}

	if c := strings.Compare(r.RR.String(), r2.RR.String()); c != 0 {
		return c
	}

	if c := strings.Compare(r.Flags, r2.Flags); c != 0 {
		return c
	}

	// Comments and string representation are the same
	return 0
}

// Zone is a collection of related Records
type Zone []*Record

func (z Zone) String() string {
	strs := make([]string, len(z))
	for i := range z {
		strs[i] = z[i].String()
	}
	return strings.Join(strs, "\n")
}

// ByRR is a Zone ordered by it's resource records.
type ByRR Zone

// Len implements Sorter for Zone
func (z ByRR) Len() int { return len(z) }

// Swap implements Sorter for Zone
func (z ByRR) Swap(i, j int) { z[i], z[j] = z[j], z[i] }

// Less implements Sorter for Zone
func (z ByRR) Less(i, j int) bool {
	return z.Compare(i, j) < 0
}

// Compare compares two elements in a Zone.
func (z ByRR) Compare(i, j int) int {
	return z[i].Compare(z[j])
}

// Dedupe z , z must already be sorted.
func (z ByRR) Dedupe() ByRR {
	if len(z) <= 1 {
		return z
	}
	i := 1
	for {
		if z.Compare(i-1, i) == 0 {
			copy(z[i:], z[i+1:])
			z = z[:len(z)-1]
		}

		if i == len(z)-1 {
			break
		}
		i++
	}
	return z
}

// ParseZoneData parses the text from the provided reader into
// zone data. All errors encountered during parsing are collected
// into the err response.
func ParseZoneData(r io.Reader) (Zone, []error) {
	var errs []error
	var z Zone

	for t := range dns.ParseZone(r, "", "") {
		if t.Error != nil {
			errs = append(errs, t.Error)
			continue
		}
		if t.RR == nil {
			continue
		}
		z = append(z, &Record{RR: t.RR, Flags: t.Comment})
	}

	return z, errs
}

type bySuffix []string

func (ss bySuffix) Len() int      { return len(ss) }
func (ss bySuffix) Swap(i, j int) { ss[i], ss[j] = ss[j], ss[i] }

func (ss bySuffix) Less(i, j int) bool {
	var minLen int
	if len(ss[i]) < len(ss[j]) {
		minLen = len(ss[i])
	} else {
		minLen = len(ss[j])
	}

	for k := 0; k < minLen; k++ {
		if d := ss[i][len(ss[i])-1-k] - ss[j][len(ss[j])-1-k]; d != 0 {
			return ss[i][len(ss[i])-1-k] < ss[j][len(ss[j])-1-k]
		}
	}

	return len(ss[i]) < len(ss[j])
}

// Partition splits a zones data into separate zones based on a
// list of domains. Records are assigned to the longest matching
// domain.
func (z Zone) Partition(domains []string) map[string]Zone {
	res := map[string]Zone{}
	sort.Sort(sort.Reverse(bySuffix(domains)))

	for _, r := range z {
		for _, d := range domains {
			if strings.HasSuffix(r.RR.Header().Name, d) {
				res[d] = append(res[d], r)
				break
			}
		}
	}

	return res
}
