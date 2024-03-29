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
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// BaseDiscovererConfig is configuration common to all discoverers
type BaseDiscovererConfig struct {
	Disabled bool         `yaml:"disabled" json:"disabled"`
	Template JSONTemplate `yaml:"template" json:"template"`
}

// BaseProvisionerConfig is the configuration that is common to
// all provisioners
type BaseProvisionerConfig struct {
	Zone           string                  `yaml:"zone" json:"zone"`
	OwnerFlagsStrs map[string]JSONTemplate `yaml:"ownerFlags"`

	ownerFlagsOnce sync.Once
	ownerFlagsErr  error
	ownerFlags     map[string]*regexp.Regexp
}

func (bp *BaseProvisionerConfig) OwnerFlags() (map[string]*regexp.Regexp, error) {
	bp.ownerFlagsOnce.Do(func() {
		out := map[string]*regexp.Regexp{}
		for k, tmpl := range bp.OwnerFlagsStrs {
			bstr := &strings.Builder{}
			err := tmpl.Execute(bstr, nil)
			if err != nil {
				bp.ownerFlagsErr = err
				return
			}
			v := bstr.String()
			if !strings.HasPrefix(v, "^") {
				v = "^" + v
			}
			if !strings.HasSuffix(v, "$") {
				v = v + "$"
			}
			vre, err := regexp.Compile(v)
			if err != nil {
				bp.ownerFlagsErr = fmt.Errorf("invalid owner flags entry %s, %w", k, err)
				return
			}
			out[k] = vre
		}
		bp.ownerFlags = out
	})
	return bp.ownerFlags, bp.ownerFlagsErr
}

// Config describes the base configuration for dubber
type Config struct {
	Discoverers struct {
		Marathon   []MarathonConfig   `yaml:"marathon" json:"marathon"`
		Kubernetes []KubernetesConfig `yaml:"kubernetes" json:"kubernetes"`
	} `yaml:"discoverers" json:"discoverers"`
	Provisioners struct {
		Route53   []Route53Config   `yaml:"route53" json:"route53"`
		GCloudDNS []GCloudDNSConfig `yaml:"gcloud" json:"gcloud"`
	} `yaml:"provisioners" json:"provisioners"`

	XXX `json:",omitempty" yaml:",omitempty,inline"`

	DryRun       bool          `json:"-"  yaml:"-"`
	OneShot      bool          `json:"-"  yaml:"-"`
	PollInterval time.Duration `json:"-"  yaml:"-"`
}

// FromYAML creates a dubber config from a YAML config file
func FromYAML(r io.Reader) (Config, error) {
	bs := &bytes.Buffer{}
	_, err := io.Copy(bs, r)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{}
	err = yaml.Unmarshal(bs.Bytes(), &cfg)
	if err != nil {
		return Config{}, err
	}

	if len(cfg.XXX) > 0 {
		unknowns := []string{}
		for k := range cfg.XXX {
			unknowns = append(unknowns, k)
		}
		return Config{}, fmt.Errorf("unknown top level config options: %s", strings.Join(unknowns, ","))
	}

	return cfg, err
}

// XXX catches unknown Rule settings
type XXX map[string]interface{}

// BuildProvisioners returns the set of provisioners for this config
func (cfg Config) BuildProvisioners() (map[string]Provisioner, error) {
	prvs := map[string]Provisioner{}
	for _, pcfg := range cfg.Provisioners.Route53 {
		dom := pcfg.Zone
		prv := NewRoute53(pcfg)
		if _, ok := prvs[dom]; ok {
			// We should actually allow this.
			return nil, fmt.Errorf("zone %q managed by multiple provisioners", dom)
		}
		if cfg.DryRun {
			prvs[dom] = dryRunProvisioner{prv}
			continue
		}
		prvs[dom] = prv
	}

	for _, pcfg := range cfg.Provisioners.GCloudDNS {
		dom := pcfg.Zone
		prv := NewGCloudDNS(pcfg)
		if _, ok := prvs[dom]; ok {
			// We should actually allow this.
			return nil, fmt.Errorf("zone %q managed by multiple provisioners", dom)
		}
		if cfg.DryRun {
			prvs[dom] = dryRunProvisioner{prv}
			continue
		}
		prvs[dom] = prv
	}

	for _, p := range prvs {
		_, err := p.OwnerFlags()
		if err != nil {
			return nil, err
		}
	}
	return prvs, nil
}

// BuildDiscoveres returns the set of discoveres for this config
func (cfg Config) BuildDiscoveres() ([]Discoverer, error) {
	var ds []Discoverer
	for i := range cfg.Discoverers.Marathon {
		dcfg := cfg.Discoverers.Marathon[i]
		if dcfg.Disabled {
			continue
		}

		d, err := NewMarathon(dcfg)
		if err != nil {
			return nil, fmt.Errorf("building marathon Discoverer failed, %w", err)
		}

		ds = append(ds, Discoverer{
			StatePuller:  d,
			JSONTemplate: dcfg.Template,
		})
	}

	for i := range cfg.Discoverers.Kubernetes {
		dcfg := cfg.Discoverers.Kubernetes[i]
		if dcfg.Disabled {
			continue
		}

		d, err := NewKubernetes(dcfg)
		if err != nil {
			return nil, fmt.Errorf("building kubernetes Discoverer failed, %w", err)
		}

		ds = append(ds, Discoverer{
			StatePuller:  d,
			JSONTemplate: dcfg.Template,
		})
	}
	return ds, nil
}
