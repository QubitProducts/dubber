package dubber

import (
	"bytes"
	"regexp"
	"testing"
)

type tpChange struct {
	adds Zone
	rems Zone
}

type testProvisioner struct {
	t  *testing.T
	rz Zone
	of map[string]*regexp.Regexp
}

func (tp *testProvisioner) UpdateZone(wanted, unwanted, desired, remote Zone) error {
	tp.t.Logf("wanted:\n%s", wanted)
	tp.t.Logf("unwanted:\n%s", unwanted)
	tp.t.Logf("desired:\n%s", desired)
	tp.t.Logf("remote:\n%s", remote)
	return nil
}

func (tp *testProvisioner) GroupFlags() []string {
	return []string{"setID", "country"}
}

func (tp *testProvisioner) OwnerFlags() (map[string]*regexp.Regexp, error) {
	return tp.of, nil
}

func (tp *testProvisioner) RemoteZone() (Zone, error) {
	return tp.rz, nil
}

func TestServerReconcile(t *testing.T) {
	z, err := ParseZoneData(bytes.NewBuffer([]byte(`
; comment is ignored
thing.example.com. 10 IN A 11.11.11.11 ; comment=1 setID=2
thing.example.com. 10 IN A 12.12.12.12
`)))
	if err != nil {
		t.Fatalf("error parsing local zone, %v", err)
	}

	rz, err := ParseZoneData(bytes.NewBuffer([]byte(`thing.example.com.	10	IN	A	8.8.8.8 ; comment=1
$TTL 86400
@   IN  SOA example.com. root.example.com. (
		100   ;Serial
		3600  ;Refresh
		1800  ;Retry
	  6048      ;Expire
    8640      ;Minimum TTL
)

thing.example.com.	10	IN	A	6.6.6.6
thing2.example.com.	10	IN	A	5.5.5.5
thing.example.com.	10	IN	A	7.7.7.7 ; setID=1 comment=1
thing.example.com.	10	IN	A	8.8.8.8 ; setID=1 comment=2
thing.example.com.	10	IN	A	10.10.10.10`)))
	if err != nil {
		t.Fatalf("error parsing remote zone, %v", err)
	}

	tp := &testProvisioner{
		t:  t,
		rz: rz,
	}

	var srv *Server
	err = srv.ReconcileZone(tp, z)
	if err != nil {
		t.Fatalf("error reconciling zone, %v", err)
	}
}

func TestServerReconcile_OwnerGroup(t *testing.T) {
	z, err := ParseZoneData(bytes.NewBuffer([]byte(`
; comment is ignored
thing.example.com. 10 IN A 11.11.11.11 ; comment=1 setID=2
thing.example.com. 10 IN A 12.12.12.12
`)))
	if err != nil {
		t.Fatalf("error parsing local zone, %v", err)
	}

	rz, err := ParseZoneData(bytes.NewBuffer([]byte(`
$TTL 86400
@   IN  SOA example.com. root.example.com. (
		100   ;Serial
		3600  ;Refresh
		1800  ;Retry
	  6048      ;Expire
    8640      ;Minimum TTL
)

thing.example.com.	10	IN	A	6.6.6.6
thing2.example.com.	10	IN	A	5.5.5.5
thing.example.com.	10	IN	A	7.7.7.7 ; setID=1 comment=1
thing.example.com.	10	IN	A	10.10.10.10
thing4.example.com.	10	IN	A	9.9.9.9; setID=1
thing5.example.com.	10	IN	A	9.9.9.9; setID=2
thing6.example.com.	10	IN	A	11.11.11.11`)))
	if err != nil {
		t.Fatalf("error parsing remote zone, %v", err)
	}

	tp := &testProvisioner{
		t:  t,
		rz: rz,
		of: map[string]*regexp.Regexp{
			"setID": regexp.MustCompile("^1$"),
		},
	}

	var srv *Server
	err = srv.ReconcileZone(tp, z)
	if err != nil {
		t.Fatalf("error reconciling zone, %v", err)
	}
}
