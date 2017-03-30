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
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

type Record struct {
	dns.RR
	Flags string
}

func (r Record) String() string {
	return fmt.Sprintf("%s %s", r.RR, r.Flags)
}

type Zone []Record

type ByRR Zone

func (z ByRR) Len() int      { return len(z) }
func (z ByRR) Swap(i, j int) { z[i], z[j] = z[j], z[i] }
func (z ByRR) Less(i, j int) bool {
	return z.Compare(i, j) < 0
}

func (z ByRR) Compare(i, j int) int {
	hi, hj := z[i].Header(), z[j].Header()
	if c := strings.Compare(hi.Name, hj.Name); c != 0 {
		return c
	}

	if hi.Ttl != hj.Ttl {
		return int(hi.Ttl - hj.Ttl)
	}

	if hi.Class != hj.Class {
		return int(hi.Class - hj.Class)
	}

	if hi.Rrtype != hj.Rrtype {
		return int(hi.Rrtype - hj.Rrtype)
	}

	if c := strings.Compare(z[i].String(), z[j].String()); c != 0 {
		return c
	}

	if c := strings.Compare(z[i].Flags, z[j].Flags); c != 0 {
		return c
	}

	return 0
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
