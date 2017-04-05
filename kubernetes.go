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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesConfig describes configuration options for
// the marathon discoverer.
type KubernetesConfig struct {
	BaseDiscovererConfig `json:",omitempty" yaml:",omitempty,inline"`
	FileName             string `json:"kubeconfig" yaml:"kubeconfig"`
	XXX                  `json:",omitempty" yaml:",omitempty,inline"`
}

// KubernetesState holds the state information we will pass to the configuration
// template.
type KubernetesState struct {
	Nodes     []v1.Node
	Ingresses []v1beta1.Ingress
	Services  []v1.Service
	Endpoints []v1.Endpoints
}

// Kubernetes implements discovery of applications and
// dns names from https://github.com/mesosphere/marathon
type Kubernetes struct {
	*kubernetes.Clientset

	sync.Mutex
	data *KubernetesState
}

// NewKubernetes creates a new marathon discoverer
func NewKubernetes(cfg KubernetesConfig) (*Kubernetes, error) {
	config, err := clientcmd.BuildConfigFromFlags("", cfg.FileName)
	if err != nil {
		return nil, err
	}
	// for now
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Kubernetes{Clientset: clientset}, err
}

// StatePull watches, or polls, marathon for new applications.
// Any matching the requires constraints are returned.
// THe first call to Discover returns all the known apps,
// Subsequent calls block until an individial update is found.
func (m *Kubernetes) StatePull(ctx context.Context) (State, error) {
	m.Lock()
	m.Unlock()

	nodesL, err := m.Clientset.Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ingsL, err := m.Clientset.Ingresses(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	svcsL, err := m.Clientset.Services(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	epsL, err := m.Clientset.Endpoints(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return &KubernetesState{
		Nodes:     nodesL.Items,
		Ingresses: ingsL.Items,
		Services:  svcsL.Items,
		Endpoints: epsL.Items,
	}, nil
}
