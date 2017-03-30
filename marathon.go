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
	"net/url"
	"sync"

	marathon "github.com/gambol99/go-marathon"
)

// MarathonConfig describes configuration options for
// the marathon discoverer.
type MarathonConfig struct {
	BaseDiscovererConfig `json:",omitempty" yaml:",omitempty,inline"`
	Endpoint             []string `json:"endpoints" yaml:"endpoints"`
	BasicAuth            struct {
		Username string `json:"username" yaml:"username"`
		Password string `json:"password" yaml:"password"`
	} `json:"basic_auth" yaml:"basic_auth"`
	XXX `json:",omitempty" yaml:",omitempty,inline"`
}

type MarathonState struct {
	Applications map[string]marathon.Application
	Tasks        map[string]marathon.Task
}

// Marathon implements discovery of applications and
// dns names from https://github.com/mesosphere/marathon
type Marathon struct {
	marathon.Marathon

	sync.Mutex
	data *MarathonState
}

// NewMarathon creates a new marathon discoverer
func NewMarathon(cfg MarathonConfig) (*Marathon, error) {
	config := marathon.NewDefaultConfig()
	config.URL = cfg.Endpoint[0]
	config.EventsTransport = marathon.EventsTransportSSE
	config.HTTPBasicAuthUser = cfg.BasicAuth.Username
	config.HTTPBasicPassword = cfg.BasicAuth.Password

	mc, err := marathon.NewClient(config)
	return &Marathon{Marathon: mc}, err
}

// Discover watches, or polls, marathon for new applications.
// Any matching the requires constraints are returned.
// THe first call to Discover returns all the known apps,
// Subsequent calls block until an individial update is found.
func (m *Marathon) Discover(ctx context.Context) (State, error) {
	m.Lock()
	m.Unlock()

	if m.data != nil {
		return m.data, nil
	}

	apps, err := m.Marathon.Applications(url.Values{})
	if err != nil {
		return nil, err
	}
	data := &MarathonState{Applications: map[string]marathon.Application{}}
	for i := range apps.Apps {
		data.Applications[apps.Apps[i].ID] = apps.Apps[i]
	}
	m.data = data
	return m.data, nil
}
