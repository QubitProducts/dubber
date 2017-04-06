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
	"context"
	"time"

	"github.com/golang/glog"
)

// Run process the configuration, passing updates form discoverers,
// managing state, and request action from provisioners.
func Run(ctx context.Context, cfg Config) error {
	provs, err := cfg.BuildProvisioners()
	if err != nil {
		return err
	}

	var provisionZones []string
	for k := range provs {
		provisionZones = append(provisionZones, k)
	}

	ds, err := cfg.BuildDiscoveres()
	if err != nil {
		return err
	}

	type update struct {
		i int
		z Zone
	}
	upds := make(chan update)

	// Launch the discoverers
	for i, d := range ds {
		go func(i int, d Discoverer) {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					z, err := d.Discover(ctx)
					if err != nil {
						glog.Info("error", err)
						return
					}
					upds <- update{i, z}
				}
			}
		}(i, d)
	}

	dzones := make([]Zone, len(ds))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case up := <-upds:
			dzones[up.i] = up.z

			var fullZone Zone
			for i := range dzones {
				fullZone = append(fullZone, dzones[i]...)
			}

			zones := fullZone.Partition(provisionZones)

			for zn, newzone := range zones {
				p, ok := provs[zn]
				if !ok {
					glog.V(1).Infof("no provisioner for zone %q\n", zn)
					continue
				}
				err := ReconcileZone(p, newzone, cfg.DryRun)
				if err != nil {
					glog.Infof(err.Error())
				}
			}
		}
	}
}
