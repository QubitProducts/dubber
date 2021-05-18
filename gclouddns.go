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
	"fmt"
	"sort"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	gdns "google.golang.org/api/dns/v1beta2"
	klog "k8s.io/klog/v2"
)

// GCloudDNSConfig describes the settings required for controlling a Google
// Cloud DNS zone.
type GCloudDNSConfig struct {
	BaseProvisionerConfig `json:",omitempty,inline" yaml:",omitempty,inline"`
	Project               string `yaml:"project" json:"project"`
	ZoneID                string `yaml:"zoneID" json:"zoneID"`
}

// GCloudDNS is an Google Cloud DNS provider.
type GCloudDNS struct {
	GCloudDNSConfig

	svc *gdns.Service
}

// NewGCloudDNS creates a gcloud dns provisioner.
func NewGCloudDNS(cfg GCloudDNSConfig) *GCloudDNS {
	ctx := context.Background()

	svc, err := gdns.NewService(ctx)
	if err != nil {
		klog.Fatalf("failed to create gcloud DNS client")
	}

	return &GCloudDNS{
		GCloudDNSConfig: cfg,
		svc:             svc,
	}
}

// GroupFlags is empty for GCloud DNS
func (r *GCloudDNS) GroupFlags() []string {
	return []string{""}
}

// RemoteZone creates a Zone from a GCloudDNS Zone.
func (r *GCloudDNS) RemoteZone() (Zone, error) {
	return zoneFromGCloudDNS(r.svc, r.Project, r.ZoneID)
}

// UpdateZone updates a GCloudDNS zone, removing the unwanted records, and
// adding any unwanted records.
func (r *GCloudDNS) UpdateZone(wanted, unwanted, desired, remote Zone) error {
	var err error

	change := gdns.Change{}

	// FIX(tcm): This relies on the SOA being the last entry in the wanted and unwanted
	// zones.
	for key, recs := range (Zone{unwanted[len(unwanted)-1]}).Group([]string{}) {
		rs, err := recordToGDNSRRS(key, recs)
		if err != nil {
			klog.Errorf("error %s", err)
			return errors.Wrap(err, "generating soa Deletion record")
		}
		change.Deletions = append(change.Deletions, rs)
	}
	for key, recs := range (Zone{wanted[len(wanted)-1]}).Group([]string{}) {
		rs, err := recordToGDNSRRS(key, recs)
		if err != nil {
			klog.Errorf("error %s", err)
			return errors.Wrap(err, "generating soa Deletion record")
		}
		change.Additions = append(change.Additions, rs)
	}

	dgs := desired.Group([]string{})
	rgs := remote.Group([]string{})

	for key, recs := range dgs {
		if rrecs, ok := rgs[key]; ok {
			rs, err := recordToGDNSRRS(key, rrecs)
			if err != nil {
				klog.Errorf("error %s", err)
				return errors.Wrap(err, "generating updating Deletion record")
			}

			klog.V(1).Infof("gcloud deletion: %v", *rs)
			change.Deletions = append(change.Deletions, rs)
		}

		rs, err := recordToGDNSRRS(key, recs)

		if err != nil {
			klog.Errorf("error %s", err)
			return errors.Wrap(err, "generating updating Additions record")
		}

		klog.V(1).Infof("gcloud addition: %v", *rs)
		change.Additions = append(change.Additions, rs)
	}

	resp, err := r.svc.Changes.Create(r.Project, r.ZoneID, &change).Do()
	if err != nil {
		return err
	}

	klog.V(1).Infof("Change succeeded:\n %v", resp)

	return nil
}

func zoneFromGCloudDNS(svc *gdns.Service, project, zone string) (Zone, error) {
	ctx := context.Background()
	var recs []*gdns.ResourceRecordSet

	err := svc.ResourceRecordSets.List(project, zone).Pages(ctx, func(rs *gdns.ResourceRecordSetsListResponse) error {
		recs = append(recs, rs.Rrsets...)
		return nil
	})
	if err != nil {
		klog.Errorf("error %#v", err)
		return nil, err
	}

	var z Zone
	for i := range recs {
		newrs, err := gdnsRRSToRecord(recs[i])
		if err != nil {
			return nil, errors.Wrapf(err, "failed rendering record for %#v", recs[i])
		}

		z = append(z, newrs...)
	}

	sort.Sort(ByRR(z))

	return z, nil
}

func gdnsRRSToRecord(r *gdns.ResourceRecordSet) (Zone, error) {
	var res Zone
	var err error
	flags := RecordFlags{}

	for i := range r.Rrdatas {
		rr := r.Rrdatas[i]
		str := fmt.Sprintf("%s %d IN %s %s", r.Name, r.Ttl, r.Type, rr)
		drr, err := dns.NewRR(str)
		if err != nil {
			klog.Infof("failed parsing record %q, %v", str, err)
			continue
		}

		res = append(res, &Record{RR: drr, Flags: flags})
	}

	return res, err
}

func recordToGDNSRRS(key RecordSetKey, zone Zone) (*gdns.ResourceRecordSet, error) {
	gr := &gdns.ResourceRecordSet{}
	gr.Name = key.Name
	rrtype, ok := dns.TypeToString[key.Rrtype]
	if !ok {
		return nil, fmt.Errorf("unknown dns.Rtype %d", key.Rrtype)
	}
	gr.Type = rrtype

	for _, r := range zone {
		gr.Rrdatas = append(gr.Rrdatas, r.RR.String()[len(r.Header().String()):])
		if key.Rrtype != dns.TypeCNAME {
			gr.Ttl = int64(r.Header().Ttl)
		}
	}

	return gr, nil
}
