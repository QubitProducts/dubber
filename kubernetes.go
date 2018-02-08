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

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesConfig describes configuration options for
// the marathon discoverer.
type KubernetesConfig struct {
	BaseDiscovererConfig `json:",omitempty" yaml:",omitempty,inline"`
	FileName             string `json:"kubeconfig" yaml:"kubeconfig"`
	Context              string `json:"context" yaml:"context"`
	XXX                  `json:",omitempty" yaml:",omitempty,inline"`
}

// KubernetesState holds the state information we will pass to the configuration
// template.
type KubernetesState struct {
	Nodes     map[string]v1.Node
	Ingresses map[string]v1beta1.Ingress
	Services  map[string]v1.Service
	Endpoints map[string]v1.Endpoints
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
	var err error
	var config *rest.Config

	if cfg.FileName != "" {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{CurrentContext: cfg.Context}

		if cfg.Context != "" {
			glog.Infof("Building kube client for context %q from %s", cfg.Context, cfg.FileName)
		} else {
			glog.Infof("Building kube client for default context  from %s", cfg.FileName)
		}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
	} else {
		glog.Info("Building in-cluster kube client")
		config, err = rest.InClusterConfig()
	}

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

	nodesM := map[string]v1.Node{}
	nodesL, err := m.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, n := range nodesL.Items {
		nodesM[n.ObjectMeta.Name] = n
	}

	ingsM := map[string]v1beta1.Ingress{}
	ingsL, err := m.Extensions().Ingresses(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, i := range ingsL.Items {
		ingsM[i.ObjectMeta.Name] = i
	}

	svcsM := map[string]v1.Service{}
	svcsL, err := m.Core().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, s := range svcsL.Items {
		svcsM[s.ObjectMeta.Name] = s
	}

	epsM := map[string]v1.Endpoints{}
	epsL, err := m.Core().Endpoints(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, e := range epsL.Items {
		epsM[e.ObjectMeta.Name] = e
	}

	return &KubernetesState{
		Nodes:     nodesM,
		Ingresses: ingsM,
		Services:  svcsM,
		Endpoints: epsM,
	}, nil
}
