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
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
)

type Route53Config struct {
	BaseProvisionerConfig `json:",omitempty" yaml:",omitempty,inline"`
}

type Route53 struct {
	Route53Config
}

func NewRoute53(cfg Route53Config) *Route53 {
	return &Route53{cfg}
}

func (r *Route53) RemoteZone() (Zone, error) {
	return r.zoneFromRoute53(r.Zone)
}

func (r *Route53) UpdateZone(wanted, unwanted Zone) error {
	log.Println("wanted: ", wanted)
	log.Println("unwanted: ", unwanted)
	return nil
}

func (r *Route53) zoneFromRoute53(name string) (Zone, error) {
	sess := session.Must(session.NewSession())
	svc := route53.New(sess)

	params := &route53.ListHostedZonesByNameInput{
		DNSName:  aws.String(name),
		MaxItems: aws.String("1"),
	}
	resp, err := svc.ListHostedZonesByName(params)
	if err != nil {
		return nil, err
	}

	if len(resp.HostedZones) == 0 {
		return nil, fmt.Errorf("uknown zone %s", name)
	}

	var awsrecs []*route53.ResourceRecordSet
	lrrsparams := &route53.ListResourceRecordSetsInput{HostedZoneId: resp.HostedZones[0].Id}
	// Example iterating over at most 3 pages of a ListResourceRecordSets operation.
	err = svc.ListResourceRecordSetsPages(lrrsparams,
		func(page *route53.ListResourceRecordSetsOutput, lastPage bool) bool {
			awsrecs = append(awsrecs, page.ResourceRecordSets...)
			return !lastPage
		})

	var z Zone
	for i := range awsrecs {
		newrs, err := awsRRToRecord(awsrecs[i])
		if err != nil {
			return nil, errors.Wrapf(err, "failed rendering record for %#v", awsrecs[i])
		}

		z = append(z, newrs...)
	}

	sort.Sort(ByRR(z))

	return z, nil
}

func awsRRToRecord(r53 *route53.ResourceRecordSet) (Zone, error) {
	var res Zone
	var err error
	flags := RecordFlags{}
	if r53.SetIdentifier != nil {
		flags["route53.SetID"] = *r53.SetIdentifier
	}

	if r53.Weight != nil {
		flags["route53.Weight"] = strconv.Itoa(int(*r53.Weight))
	}

	for i := range r53.ResourceRecords {
		rr := r53.ResourceRecords[i]
		str := fmt.Sprintf("%s %d IN %s %s", *r53.Name, *r53.TTL, *r53.Type, *rr.Value)
		drr, err := dns.NewRR(str)
		if err != nil {
			log.Println(err)
			continue
		}

		res = append(res, &Record{RR: drr, Flags: flags})
	}

	if r53.AliasTarget != nil {
		str := fmt.Sprintf("%s 0 IN %s 0.0.0.0", *r53.Name, *r53.Type)
		drr, err := dns.NewRR(str)
		if err != nil {
			return res, err
		}
		flags["route53.Alias"] = *r53.AliasTarget.HostedZoneId + ":" + *r53.AliasTarget.DNSName

		res = append(res, &Record{RR: drr, Flags: flags})
	}
	return res, err
}
