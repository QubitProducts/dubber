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
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/golang/glog"
	"github.com/pkg/errors"
)

// State is any data passed from the discoverer to the
// template to generate DNS records
type State interface{}

// StatePuller reads the state from the remote service.
// The call should block until an updated state is
// available
type StatePuller interface {
	StatePull(context.Context) (State, error)
}

// Discoverer combined zone data and state into a Zone
type Discoverer struct {
	StatePuller
	State interface{}
	JSONTemplate
}

// Discover pulls the state from a StatePuller and renders the
// state into Zone data
func (d *Discoverer) Discover(ctx context.Context) (Zone, error) {
	state, err := d.StatePull(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pull state")
	}

	glog.V(2).Infof("template state input: %#v\n", state)

	buf := &bytes.Buffer{}
	err = d.Execute(buf, state)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render zone")
	}

	glog.V(1).Info("template output:\n", buf.String())

	z, err := ParseZoneData(buf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse zone")
	}
	d.State = state
	return z, nil
}

// JSONTemplate provides a means of directly unmarshaling a template
type JSONTemplate struct {
	*template.Template
}

// MarshalYAML implements the yaml Marshaler interface for JSON template
func (t JSONTemplate) MarshalYAML() (interface{}, error) {
	bs, err := t.MarshalJSON()
	return string(bs), err
}

// MarshalJSON implements the yaml Marshaler interface for JSON template
func (t JSONTemplate) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", t.Template.Tree.Root.String())), nil
}

// UnmarshalYAML implements the yaml Unmarshaler interface for JSON
// Regex
func (t *JSONTemplate) UnmarshalYAML(unmarshal func(interface{}) error) error {
	str := ""
	if err := unmarshal(&str); err != nil {
		return err
	}
	jstr := fmt.Sprintf("%q", str)
	return t.UnmarshalJSON([]byte(jstr))
}

// UnmarshalJSON implements the yaml Unmarshaler
// interface for JSON Regex
func (t *JSONTemplate) UnmarshalJSON(bs []byte) error {
	str := ""
	if err := json.Unmarshal(bs, &str); err != nil {
		return err
	}
	tmpl := template.Must(template.New("base").Funcs(sprig.TxtFuncMap()).Parse(str))
	*t = JSONTemplate{tmpl}
	return nil
}
