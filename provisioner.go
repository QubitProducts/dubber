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

import "sort"

type Provisioner interface {
	RemoteZone() (Zone, error)
	UpdateZone(remove, add Zone) error
}

func ReconcileZone(p Provisioner, desired Zone) error {
	dgroups := desired.Group()

	remz, err := p.RemoteZone()
	if err != nil {
		return err
	}

	rgroups := remz.Group()

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
		if len(wanted) > 0 || len(unwanted) > 0 {
			p.UpdateZone(wanted, unwanted)
		}
	}
	return nil
}
