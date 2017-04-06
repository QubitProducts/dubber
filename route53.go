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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/golang/glog"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
)

type Route53Config struct {
	BaseProvisionerConfig `json:",omitempty" yaml:",omitempty,inline"`
	ZoneID                string `json:"zoneid,omitempty" yaml:"zoneid,omitempty"`
}

type Route53 struct {
	sync.Mutex
	Route53Config
}

func NewRoute53(cfg Route53Config) *Route53 {
	return &Route53{Route53Config: cfg}
}

func (r *Route53) RemoteZone() (Zone, error) {
	var err error
	r.Lock()
	defer r.Unlock()
	if r.ZoneID == "" {
		r.ZoneID, err = zoneIdFromRoute53(r.Zone)
		if err != nil {
			return nil, errors.Wrap(err, "could not retrieve remote zone")
		}
	}

	return zoneFromRoute53(r.ZoneID)
}

func (r *Route53) UpdateZone(wanted, unwanted Zone) error {
	var err error
	if r.ZoneID == "" {
		r.ZoneID, err = zoneIdFromRoute53(r.Zone)
		if err != nil {
			return errors.Wrap(err, "could not update zone")
		}
	}

	changes := route53.ChangeBatch{
		Comment: aws.String(fmt.Sprintf("dubber did it... %s", time.Now())),
	}

	for _, uw := range unwanted {
		awsrrs, err := recordToAWSRRS(uw)
		if err != nil {
			return errors.Wrap(err, "generating updating DELETE record")
		}

		change := route53.Change{
			Action:            aws.String("DELETE"),
			ResourceRecordSet: awsrrs,
		}
		changes.Changes = append(changes.Changes, &change)
	}

	for _, w := range wanted {
		awsrrs, err := recordToAWSRRS(w)
		if err != nil {
			return errors.Wrap(err, "generating CREATE record")
		}
		change := route53.Change{
			Action:            aws.String("CREATE"),
			ResourceRecordSet: awsrrs,
		}
		changes.Changes = append(changes.Changes, &change)
	}

	glog.V(1).Infof("Route53 Changes to %s: %s", r.ZoneID, changes)

	sess := session.Must(session.NewSession())
	svc := route53.New(sess)
	params := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.ZoneID),
		ChangeBatch:  &changes,
	}
	// Example iterating over at most 3 pages of a ListResourceRecordSets operation.
	out, err := svc.ChangeResourceRecordSets(params)
	if err != nil {
		return err
	}

	glog.V(1).Infof("Change succeeded:\n %s", out)

	return nil
}

func zoneIdFromRoute53(name string) (string, error) {
	sess := session.Must(session.NewSession())
	svc := route53.New(sess)

	params := &route53.ListHostedZonesByNameInput{
		DNSName:  aws.String(name),
		MaxItems: aws.String("1"),
	}
	resp, err := svc.ListHostedZonesByName(params)
	if err != nil {
		return "", err
	}

	if len(resp.HostedZones) == 0 {
		return "", fmt.Errorf("uknown zone %s", name)
	}

	if len(resp.HostedZones) > 1 {
		return "", fmt.Errorf("too many zones found for %s (%d zones)", name, len((resp.HostedZones)))
	}

	return *resp.HostedZones[0].Id, nil
}

func zoneFromRoute53(zoneID string) (Zone, error) {
	sess := session.Must(session.NewSession())
	svc := route53.New(sess)

	var awsrecs []*route53.ResourceRecordSet
	lrrsparams := &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)}
	// Example iterating over at most 3 pages of a ListResourceRecordSets operation.
	if err := svc.ListResourceRecordSetsPages(lrrsparams,
		func(page *route53.ListResourceRecordSetsOutput, lastPage bool) bool {
			awsrecs = append(awsrecs, page.ResourceRecordSets...)
			return !lastPage
		}); err != nil {
		return nil, err
	}

	var z Zone
	for i := range awsrecs {
		newrs, err := awsRRSToRecord(awsrecs[i])
		if err != nil {
			return nil, errors.Wrapf(err, "failed rendering record for %#v", awsrecs[i])
		}

		z = append(z, newrs...)
	}

	sort.Sort(ByRR(z))

	return z, nil
}

func awsRRSToRecord(r53 *route53.ResourceRecordSet) (Zone, error) {
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
			glog.Info("failed parsing record %q, %v", str, err)
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

func recordToAWSRRS(r *Record) (*route53.ResourceRecordSet, error) {
	r53 := &route53.ResourceRecordSet{}
	r53.Name = aws.String(r.Header().Name)
	rrtype, ok := dns.TypeToString[r.Header().Rrtype]
	if !ok {
		return nil, fmt.Errorf("unknown dns.Rtype %d", r.Header().Rrtype)
	}
	r53.Type = aws.String(rrtype)

	if aliasStr, ok := r.Flags["route53.Alias"]; ok {
		aliasStrs := strings.SplitN(aliasStr, ":", 2)
		if len(aliasStrs) != 2 {
			return nil, fmt.Errorf("could not parse alias, must be HOSTEDZONEID:NAME %d", r.Header().Rrtype)
		}
		aliasZone := aliasStrs[0]
		aliasName := aliasStrs[1]
		r53.AliasTarget = &route53.AliasTarget{
			HostedZoneId:         aws.String(aliasZone),
			DNSName:              aws.String(aliasName),
			EvaluateTargetHealth: aws.Bool(false),
		}
	}

	if setIDStr, ok := r.Flags["route53.SetID"]; ok {
		r53.SetIdentifier = &setIDStr
	}

	if weighStr, ok := r.Flags["route53.Weight"]; ok {
		w, err := strconv.Atoi(weighStr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse weight as int")
		}

		wint64 := int64(w)
		r53.Weight = &wint64
	}

	if r53.AliasTarget != nil {
		return r53, nil
	}

	r53.ResourceRecords = []*route53.ResourceRecord{
		{Value: aws.String(r.RR.String()[len(r.Header().String()):])},
	}

	if r.Header().Rrtype != dns.TypeCNAME {
		r53.TTL = aws.Int64(int64(r.Header().Ttl))
	}

	return r53, nil
}
