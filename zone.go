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
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

// RecordFlags is a set of KV pairs, parsed from the comments of a record.
// They are used to pass hints to the provisioners.
type RecordFlags map[string]string

// ParseRecordFlags parses simple K=V pairs from a comment on a record.
// Any bare words are included and are assumed to have a "" value.
func ParseRecordFlags(str string) (RecordFlags, error) {
	var res RecordFlags

	scan := bufio.NewScanner(strings.NewReader((str)))
	scan.Split(bufio.ScanWords)
	for scan.Scan() {
		vs := strings.SplitN(scan.Text(), "=", 2)
		k := vs[0]
		v := ""
		if len(vs) == 2 {
			v = vs[1]
		}
		if res == nil {
			res = make(RecordFlags)
		}
		res[k] = v
	}

	if err := scan.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

// String implements Stringer for a RecordFlags, rendering
// the strings in sorted order
func (rf RecordFlags) String() string {
	strs := []string{}

	ks := []string{}
	for k := range rf {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		str := k
		if len(rf[k]) > 0 {
			str += fmt.Sprintf("=%s", rf[k])
		}
		strs = append(strs, str)
	}

	return strings.Join(strs, " ")
}

// Compare two sets of RecordFlags. Retursn  > 0 if
// mthere are more flags in rf2 than rf. If the number
// of flags is the same, the String representations are
// compared.
func (rf RecordFlags) Compare(rf2 RecordFlags) int {
	if c := len(rf) - len(rf2); c != 0 {
		return c
	}

	return strings.Compare(rf.String(), rf2.String())
}

// Record represents a DNS record we wish to be present,
// along with a Flags string which may contain hints to the
// provisioner
type Record struct {
	dns.RR
	Flags RecordFlags
}

// String implements fmt.Stringer for a Record
func (r *Record) String() string {
	if r == nil {
		return ""
	}
	str := r.RR.String()
	if len(r.Flags) != 0 {
		str += " ; " + r.Flags.String()
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

	if c := r.Flags.Compare(r2.Flags); c != 0 {
		return c
	}

	// Comments and string representation are the same
	return 0
}

// Zone is a collection of related Records
type Zone []*Record

// String renders the text version of the Zone data
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

// ZoneError is the set of errors seen when parsing Zone
// data
type ZoneError []error

// Error implenebts the Error interface for a set of errors
// found when parsing a zone.
func (z ZoneError) Error() string {
	var strs []string

	for _, e := range z {
		strs = append(strs, e.Error())
	}
	if len(strs) > 0 {
		strs = append([]string{fmt.Sprintf("%d errors while processing zone:\n", len(strs))}, strs...)
	}
	return strings.Join(strs, "\n")
}

// ParseZoneData parses the text from the provided reader into
// zone data. All errors encountered during parsing are collected
// into the err response.
func ParseZoneData(r io.Reader) (Zone, error) {
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
		var flags RecordFlags
		if len(t.Comment) > 1 {
			var err error
			flags, err = ParseRecordFlags(t.Comment[1:])
			if err != nil {
				errs = append(errs, t.Error)
				continue
			}
		}
		z = append(z, &Record{RR: t.RR, Flags: flags})
	}

	if len(errs) > 0 {
		return nil, ZoneError(errs)
	}
	return z, nil
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

// Diff enumerates the differences between two zones. Both zones should be
// sorted before calling.
// The first return argument are those items only in the original zone
// The Second return argument are those items common to both zones
// The Third return argument are those items present only in the argument zone
func (z Zone) Diff(z2 Zone) (Zone, Zone, Zone) {
	var lz, cz, rz Zone

	cz = lcsZone(z, z2)

	j := 0
	for i := 0; i < len(z); i++ {
		if j < len(cz) && z[i].Compare(cz[j]) == 0 {
			j++
			continue
		}
		lz = append(lz, z[i])
	}

	j = 0
	for i := 0; i < len(z2); i++ {
		if j < len(cz) && z2[i].Compare(cz[j]) == 0 {
			j++
			continue
		}
		rz = append(rz, z2[i])
	}

	return lz, cz, rz
}

func lcsZone(a, b Zone) Zone {
	lena := len(a)
	lenb := len(b)

	table := make([][]int, lena+1)
	for i := range table {
		table[i] = make([]int, lenb+1)
	}

	for i := 0; i <= lena; i++ {
		table[i][0] = 0
	}

	for j := 0; j <= lenb; j++ {
		table[0][j] = 0
	}

	for i := 1; i <= lena; i++ {
		for j := 1; j <= lenb; j++ {
			if a[i-1].Compare(b[j-1]) == 0 {
				table[i][j] = table[i-1][j-1] + 1
			} else {
				table[i][j] = max(table[i-1][j], table[i][j-1])
			}
		}
	}

	return back(table, a, b, lena, lenb)
}

func max(more ...int) int {
	max := more[0]
	for _, elem := range more {
		if max < elem {
			max = elem
		}
	}
	return max
}

func back(table [][]int, a, b Zone, i, j int) Zone {
	if i == 0 || j == 0 {
		return nil
	} else if a[i-1].Compare(b[j-1]) == 0 {
		return append(back(table, a, b, i-1, j-1), a[i-1])
	} else {
		if table[i][j-1] > table[i-1][j] {
			return back(table, a, b, i, j-1)
		}
		return back(table, a, b, i-1, j)
	}
}

type lcsTable [][]int

func (t lcsTable) String() string {
	str := ""
	for i := range t {
		for j := range t[i] {
			str += strconv.Itoa(t[i][j])
			str += " "
		}
		str += "\n"
	}
	return str
}

// FindSet finds the set of records matching the provided name, class and type
func (z Zone) FindSet(name string, class uint16, rrtype uint16) Zone {
	var nz Zone
	for _, rr := range z {
		if rr.Header().Name == name &&
			rr.Header().Class == class &&
			rr.Header().Rrtype == rrtype {
			nz = append(nz, rr)
		}
	}

	return nz
}

// RecordSetKey is used to group records by name, type and class, along
// with any grouping keys.
type RecordSetKey struct {
	Name       string
	Class      uint16
	Rrtype     uint16
	GroupFlags string
}

// Group all the records by Name,Class and Type, and a set of
// grouping flags.
func (z Zone) Group(groupFlags []string) map[RecordSetKey]Zone {
	res := map[RecordSetKey]Zone{}

	for _, rr := range z {
		var flags []string
		for _, f := range groupFlags {
			if v, ok := rr.Flags[f]; ok {
				flags = append(flags, fmt.Sprintf("%s=%q", f, v))
			}
		}
		k := RecordSetKey{
			Name:       rr.Header().Name,
			Class:      rr.Header().Class,
			Rrtype:     rr.Header().Rrtype,
			GroupFlags: strings.Join(flags, " "),
		}
		rz := res[k]
		rz = append(rz, rr)
		res[k] = rz
	}

	return res
}
