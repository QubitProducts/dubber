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
	"io"

	yaml "gopkg.in/yaml.v2"
)

// Config describes the base configuration for dubber
type Config struct {
	Discoverers struct {
		Marathon []MarathonConfig
	} `yaml:"discoverers" json:"discoverers"`
	Provisioners struct {
		Route53 []Route53Config
	} `yaml:"provisioners" json:"provisioners"`
	XXX `json:",omitempty" yaml:",omitempty,inline"`
}

// ConfigFromYAML creates a dubber config from a YAML config file
func FromYAML(r io.Reader) (Config, error) {
	bs := &bytes.Buffer{}
	io.Copy(bs, r)
	cfg := Config{}
	err := yaml.Unmarshal(bs.Bytes(), &cfg)
	return cfg, err
}

// XXX catches unknown Rule settings
type XXX map[string]interface{}
