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
	"bytes"
	"context"
	"log"
	"sort"
	"time"

	"github.com/miekg/dns"
)

// Run process the configuration, passing updates form discoverers,
// managing state, and request action from provisioners.
func Run(ctx context.Context, cfg Config) {
	provs := map[string]Provisioner{}
	for _, pcfg := range cfg.Provisioners.Route53 {
		if _, ok := provs[pcfg.BaseProvisionerConfig.Zone]; ok {
			log.Printf("provisioner for %s already defined", pcfg.Zone)
			continue
		}
		provs[pcfg.BaseProvisionerConfig.Zone] = NewRoute53(pcfg)
	}
	log.Printf("provisioner: %#v", provs)

	for _, dcfg := range cfg.Discoverers.Marathon {
		d, err := NewMarathon(dcfg)
		if err != nil {
			log.Printf("failed to create discoverer, %v", err)
			continue
		}

		for {
			state, err := d.Discover(ctx)
			if err != nil {
				log.Printf("failed to run discoverer, %v", err)
				continue
			}

			buf := &bytes.Buffer{}
			if err := dcfg.Template.Execute(buf, state); err != nil {
				log.Printf("failed to execute template, %v", err)
				continue
			}
			var z Zone
			for t := range dns.ParseZone(buf, "", "") {
				if t.Error != nil {
					log.Printf("errors in zone config, %v", t.Error)
					continue
				}
				if t.RR == nil {
					continue
				}
				z = append(z, &Record{RR: t.RR, Flags: t.Comment})
			}
			sort.Sort(ByRR(z))
			z = Zone(ByRR(z).Dedupe())

			for k, v := range z.Partition([]string{"qubit.com.", "qutics.com."}) {
				p, ok := provs[k]
				if !ok {
					log.Printf("no provisioner for %s", k)
					continue
				}
				p.EnsureState(v)
			}

			time.Sleep(10 * time.Second)
		}
	}
}
