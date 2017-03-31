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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
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

func (r *Route53) EnsureState(z Zone) {
	remz, err := r.zoneFromRoute53(r.Zone)
	log.Println(remz, err)
	return
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
		r, err := dns.NewRR(fmt.Sprintf("%s 10 IN MX 10 0.0.0.0", *awsrecs[i].Name))
		if err != nil {
			log.Println("err: ", err.Error())
			continue
		}
		z = append(z, &Record{
			RR: r,
		})
	}

	return z, nil
}
