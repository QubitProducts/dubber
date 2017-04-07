
*WARNING* dubber is in it's infancy. It may not even be a great idea at all.

# Dubber

A tool for provisioning DNS records based on dynamic config from
various sources.

## What Does Dubber Do?

Dubber queries various sources of information (currently Marathon,
or Kubernetes) for the state of tasks and services running within
them.

This state is then passed to user supplied templates
([text/template](https://godoc.org/text/template)
with [github.com/Masterminds/sprig]()https://godoc.org/github.com/Masterminds/sprig) enabled).

Output of the template is parsed as  RFC 1035 Zone file content using
[github.com/miekg](https://godoc.org/github.com/miekg).

The resulting zone file is then passed to DNS provisioners (currently
route53, gcloud DNS coming soon), which reconcile the provided zone
content with the running zone.

Template can use the comments on a DNS record to pass hints to the
provisioner. 

At present it will only manage records that discoveres find, selective
purging of records may be available in future.

## An example

```
discoverers:
  marathon:
    - endpoints:
      -  http://marathon.mesos.example.com/api
      template: |
        {{- $publicLB := "Z1234:dualstack.exanple-public.elb.amazonaws.com."}}
        {{- $privateLB := "Z1234:dualstack.exanple-private.elb.amazonaws.com."}}
        {{- range .Applications }} 
        {{- if (index .Labels "dnsName") }}
        {{- if eq (index .Labels "externalAccess") "public" }}
        {{ index .Labels "dnsName"}} 0 A 0.0.0.0 ; route53.Alias={{$publicLB}}
        {{- else }}
        {{ index .Labels "dnsName"}} 0 A 0.0.0.0 ; route53.Alias={{$privateLB}}
        {{- end }}
        {{- end }}
        {{- end }}

provisioners:
  route53:
    - zone: example.com.
      zoneid: Z56789
```

Will create a route53 alias record, based on the dnsName and externalAccess
labels on a Narathon task.

Different disoveres can provide different data, and any number of records can be
created for different elements.

# TODO
- Watch rather than poll
- Selective poll interval
- Purging
- Possibly unify all data and pass it to a single template, rather than each
  discoverer having it's own template.
- Functions to help build the records.


