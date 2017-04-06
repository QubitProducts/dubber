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
	"log"
	"sort"

	"github.com/miekg/dns"
)

type Provisioner interface {
	RemoteZone() (Zone, error)
	UpdateZone(remove, add Zone) error
}

func ReconcileZone(p Provisioner, desired Zone, dryRun bool) error {
	dgroups := desired.Group()

	remz, err := p.RemoteZone()
	if err != nil {
		return err
	}

	rgroups := remz.Group()

	var soarr *Record
	for rgroupKey, rgroup := range rgroups {
		if rgroupKey.Rrtype != dns.TypeSOA {
			continue
		}
		if soarr != nil || len(rgroup) > 1 {
			return fmt.Errorf("multople SOA records found")
		}
		if len(rgroup) != 1 {
			return fmt.Errorf("invalid group fp SOA records, must have exactly one record, got , ", rgroup)
		}
		soarr = rgroup[0]
	}

	if soarr == nil {
		return fmt.Errorf("no SOA records found")
	}

	var allWanted, allUnwanted Zone
	for dgroupKey, dgroup := range dgroups {
		rgroup, ok := rgroups[dgroupKey]
		if !ok {
			rgroup = make(Zone, 0)
		}

		sort.Sort(ByRR(dgroup))
		sort.Sort(ByRR(rgroup))

		dgroup = Zone(ByRR(dgroup).Dedupe())
		rgroup = Zone(ByRR(rgroup).Dedupe())

		wanted, _, unwanted := dgroup.Diff(rgroup)

		allUnwanted = append(allUnwanted, unwanted...)
		allWanted = append(allWanted, wanted...)
	}

	if len(allWanted) == 0 && len(allUnwanted) == 0 {
		log.Println("nothing to do")
		return nil
	}

	// generate a new SOA record.
	soa, ok := soarr.RR.(*dns.SOA)
	if !ok {
		return fmt.Errorf("unable to cast dns.RR %q to SOA record", soa)
	}

	newsoa := *soa
	newsoa.Serial++

	allWanted = append(allWanted, &Record{RR: &newsoa})
	allUnwanted = append(allUnwanted, soarr)

	if dryRun {
		log.Println("Unwanted records to be removed:\n", allUnwanted)
		log.Println("Wanted records to be added:\n", allWanted)
		return nil
	}

	return p.UpdateZone(allWanted, allUnwanted)
}
