
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
with [github.com/Masterminds/sprig](https://godoc.org/github.com/Masterminds/sprig)) enabled).

Output of the template is parsed as  RFC 1035 Zone file content using
[github.com/miekg](https://godoc.org/github.com/miekg).

The resulting zone file is then passed to DNS provisioners (currently
route53 and gcloud DNS), which reconcile the provided zone
content with the running zone.

Template can use the comments on a DNS record to pass hints to the
provisioner.

Dubber will only delete records under the following two circumstances:
- A record of the same Name, Class, RR Type and Grouping Flag values (see below), already
  exists, and has a different value.
- No records of the same Name, Class, RR Type and Grouping Flag values (see below), were
  request, and all the flags mentioned in the ownerFlags in the provisioner configuration
  are set, and match the configured regexp of each flag.

So by settings some grouping flag on a record, (e.g. route53.SetID), to a value that can be
specific to a given instance of dubber, you can then allow dubber to delete records that match
that specific value, if they are no longer needed.

## Record Flags

Dubber uses DNS comments to translate into non-traditional DNS options supported by the provisioners.
Some flags are used to group records so that conflicting records within a group can be remove/replaced,
but conflicting records in different groups are treated a separate entitied.

### route53

- `route53.SetID`: (Grouping Flag)  Associate these records with a Set
- `route53.Weight`: Set a weight for the set
- `route53.Alias`: "HOSTEDZONEID:ALIASNAME"
- `route53.EvalTargetHealth`: "true" will enable target health evaluation

### GCloud DNS

TBD

## An example

```
discoverers:
  kubernetes:
    - template: |
      {{ $cluster := env `CLUSTER` }}
      ; From Ingresses
      {{- range $ing := .Ingresses }}
        {{- $setID := or (index $ing.ObjectMeta.Labels `route53SetId`) $cluster }}
        {{- $weight := or (index $ing.ObjectMeta.Labels `route53Weight`) `0` }}
        {{- if len $ing.Status.LoadBalancer.Ingress }}
        {{-   $sing := index $ing.Status.LoadBalancer.Ingress 0 }}
        {{-   range $rule := $ing.Spec.Rules }}
        {{-     if $rule.Host }}
        {{-       if $sing.IP }}
      {{ $rule.Host }} 60 A {{$sing.IP}}; route53.SetID={{ $setID }} route53.Weight={{ $weight }}
        {{-       else }}
      {{ $rule.Host }} CNAME {{ $sing.Hostname }};
        {{-       end }}
        {{-     end }}
        {{-   end }}
        {{- end  }}
      {{- end }}
  marathon:
    - endpoints:
      -  http://marathon.mesos.example.com/api
      template: |
        {{- $publicLB := `Z1234:dualstack.exanple-public.elb.amazonaws.com.`}}
        {{- $privateLB := `Z1234:dualstack.exanple-private.elb.amazonaws.com.`}}
        {{- range .Applications }}
        {{-   if (index .Labels `dnsName`) }}
        {{-     if eq (index .Labels `externalAccess`) `public` }}
        {{ index .Labels `dnsName`}} 0 A 0.0.0.0 ; route53.Alias={{$publicLB}}
        {{-     else }}
        {{ index .Labels `dnsName`}} 0 A 0.0.0.0 ; route53.Alias={{$privateLB}}
        {{-     end }}
        {{-   end }}
        {{- end }}

provisioners:
  route53:
    - zone: example.com.
      zoneid: Z56789
      ownerFlags:
        "route53.SetID": "{{ env `CLUSTER` }}"
```

Will create a route53 alias record, based on the dnsName and externalAccess
labels on a Narathon task.

Different disoveres can provide different data, and any number of records can be
created for different elements.

# TODO
- Watch rather than poll
- Possibly unify all data and pass it to a single template, rather than each
  discoverer having it's own template.
- Template functions to help build the records.
- Purging - possibly track the state of the records and allow purging of anything
  we created (this is somewhat achievable via the ownerFlags)


