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
	"regexp"
	"sort"

	"github.com/miekg/dns"
	klog "k8s.io/klog/v2"
)

// A Provisioner can manage a zone. RemoteZone should include exactly 1 SOA
// record. It is assumed that Zones do not change without that Serial Number
// being changed. In the event that records must be added/removed from the
// Zone retuned by RemoteZone, UpdateZone will be called with the relevant
// changes, plus an update to the SOA record. It is assumed that an update
// will fail if the SOA serial from the remote list does not match the
// SOA of the current remote zone state.
type Provisioner interface {
	RemoteZone() (Zone, error)
	UpdateZone(wanted, unwanted, desired, remote Zone) error
	GroupFlags() []string
	OwnerFlags() (map[string]*regexp.Regexp, error)
}

// ReconcileZone attempts to ensure that the set of records in the desired
// zone are present in the Provisioner's zone.
//   - Records are grouped by Name.
//   - Records from the provisioner that are not listed in the desired set
//     are ignored.
//   - Records of a given "Name, Type , Class" combination that are in the
//     remote zone, but not in the desired zone are removed.
//   - Records of a given "Name, Type , Class" combination that are in the
//     desired zone, but not in the remote zone are added.
func (srv *Server) ReconcileZone(p Provisioner, desired Zone) error {
	remz, err := p.RemoteZone()
	if err != nil {
		return err
	}

	var soarr *Record
	for _, rr := range remz {
		if rr.RR.Header().Rrtype != dns.TypeSOA {
			continue
		}
		if soarr != nil {
			return fmt.Errorf("multiple SOA records found")
		}
		soarr = rr
	}

	if soarr == nil {
		return fmt.Errorf("no SOA records found")
	}

	// generate a new SOA record.
	soa, ok := soarr.RR.(*dns.SOA)
	if !ok {
		return fmt.Errorf("unable to cast dns.RR %q to SOA record", soa)
	}

	if srv != nil {
		srv.MetricDiscoveredZoneSerial.WithLabelValues(soa.Header().Name).Set(float64(soa.Serial))
	}

	dgroups := desired.Group(p.GroupFlags())
	rgroups := remz.Group(p.GroupFlags())

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

	// unused remote groups
	var unusedGroups []RecordSetKey
	for rgroupKey := range rgroups {
		_, ok := dgroups[rgroupKey]
		if ok {
			continue
		}
		// We can ignore the error here because this string
		// was produced by us and so should always be valid
		fs, _ := ParseRecordFlags(rgroupKey.GroupFlags)
		matches := 0

		// We can ignore the error here because we've already
		// parsed these from config
		oflags, _ := p.OwnerFlags()
		if len(oflags) == 0 {
			continue
		}
		for k, rx := range oflags {
			for fk, fv := range fs {
				if k != fk {
					continue
				}
				if rx.MatchString(fv) {
					matches += 1
				}
			}
		}
		if matches != 0 && matches == len(oflags) {
			unusedGroups = append(unusedGroups, rgroupKey)
		}
	}

	for _, unusedGroupKey := range unusedGroups {
		rgroup := Zone(ByRR(rgroups[unusedGroupKey]).Dedupe())
		allUnwanted = append(allUnwanted, rgroup...)
	}

	if len(allWanted) == 0 && len(allUnwanted) == 0 {
		klog.V(1).Info("nothing to do")
		return nil
	}

	newsoa := *soa
	newsoa.Serial++

	allWanted = append(allWanted, &Record{RR: &newsoa})
	allUnwanted = append(allUnwanted, soarr)

	err = p.UpdateZone(allWanted, allUnwanted, desired, remz)
	if err == nil && srv != nil {
		srv.MetricProvisionedZoneSerial.WithLabelValues(soa.Header().Name).Set(float64(soa.Serial))
	}
	return err
}

type dryRunProvisioner struct {
	real Provisioner
}

func (p dryRunProvisioner) GroupFlags() []string {
	return p.real.GroupFlags()
}

func (p dryRunProvisioner) OwnerFlags() (map[string]*regexp.Regexp, error) {
	return p.real.OwnerFlags()
}

func (p dryRunProvisioner) RemoteZone() (Zone, error) {
	return p.real.RemoteZone()
}

func (p dryRunProvisioner) UpdateZone(allWanted, allUnwanted, desired, remote Zone) error {
	klog.V(0).Info("Unwanted records to be removed:\n", allUnwanted)
	klog.V(0).Info("Wanted records to be added:\n", allWanted)
	return nil
}
