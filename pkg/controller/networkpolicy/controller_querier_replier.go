// Copyright 2019 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package networkpolicy provides NetworkPolicyController implementation to manage
// and synchronize the Pods and Namespaces affected by Network Policies and enforce
// their rules.

package networkpolicy

import (
	networkingv1beta1 "github.com/vmware-tanzu/antrea/pkg/apis/networking/v1beta1"
	"github.com/vmware-tanzu/antrea/pkg/controller/networkpolicy/store"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	antreatypes "github.com/vmware-tanzu/antrea/pkg/controller/types"
)

type EndpointQuerier interface {
	QueryNetworkPolicies(namespace string, podName string) (*EndpointQueryResponse, error)
}

// EndpointQueryReplier is responsible for handling query requests from antctl query
type EndpointQueryReplier struct {
	networkPolicyController *NetworkPolicyController
}

// EndpointQueryResponse is the reply struct for QueryNetworkPolicies
type EndpointQueryResponse struct {
	Endpoints []Endpoint `json:"endpoints,omitempty"`
}

// Endpoint holds response information for an endpoint following a query
type Endpoint struct {
	Namespace string   `json:"namespace,omitempty"`
	Name      string   `json:"name,omitempty"`
	Policies  []Policy `json:"policies,omitempty"`
	Rules     []Rule   `json:"rules,omitempty"`
}

type PolicyRef struct {
	Namespace string    `json:"namespace,omitempty"`
	Name      string    `json:"name,omitempty"`
	UID       types.UID `json:"uid,omitempty"`
}

// Policy holds network policy information to be relayed to client following query endpoint
type Policy struct {
	PolicyRef
	selector metav1.LabelSelector `json:"selector,omitempty"`
}

// Rule holds
type Rule struct {
	PolicyRef
	Direction networkingv1beta1.Direction `json:"direction,omitempty"`
	RuleIndex int                         `json:"ruleindex,omitempty"`
	Ports     []v1.NetworkPolicyPort      `json:"ports,omitempty"`
}

// NewNetworkPolicyController returns a new *NetworkPolicyController.
func NewEndpointQueryReplier(networkPolicyController *NetworkPolicyController) *EndpointQueryReplier {
	n := &EndpointQueryReplier{
		networkPolicyController: networkPolicyController,
	}
	return n
}

//Query functions
func (eq EndpointQueryReplier) QueryNetworkPolicies(namespace string, podName string) (*EndpointQueryResponse, error) {
	// check if namespace and podName select an existing pod
	_, err := eq.networkPolicyController.podInformer.Lister().Pods(namespace).Get(podName)
	if err != nil {
		return &EndpointQueryResponse{
			Endpoints: nil,
		}, err
	}
	// create network policies categories
	applied, _, _ := make([]*antreatypes.NetworkPolicy, 0), make([]antreatypes.NetworkPolicy, 0),
		make([]antreatypes.NetworkPolicy, 0)
	// filter all policies into appropriate groups
	appliedToGroups, err := eq.networkPolicyController.appliedToGroupStore.GetByIndex(store.PodIndex, podName + "/" + namespace)
	if err != nil {
		return &EndpointQueryResponse{
			Endpoints: nil,
		}, err
	}
	// iterate through appliedToGroups to list applied policies using appliedToGroups indexer
	for _, appliedToGroup := range appliedToGroups {
		policies, err := eq.networkPolicyController.internalNetworkPolicyStore.GetByIndex(store.AppliedToGroupIndex,
			string(appliedToGroup.(*antreatypes.AppliedToGroup).UID))
		if err != nil {
			return &EndpointQueryResponse{
				Endpoints: nil,
			}, err
		}
		for _, policy := range policies {
			applied = append(applied, policy.(*antreatypes.NetworkPolicy))
		}


	}
	// make response policies
	responsePolicies := make([]Policy, 0)
	for _, internalPolicy := range applied {
		responsePolicy := Policy{
			PolicyRef: PolicyRef{
				Namespace: internalPolicy.Namespace,
				Name:      internalPolicy.Name,
				UID:       internalPolicy.UID,
			},
		}
		responsePolicies = append(responsePolicies, responsePolicy)
	}
	// make rules
	responseRules := make([]Rule, 0)
	// endpoint
	endpoint := Endpoint{
		Namespace: namespace,
		Name:      podName,
		Policies:  responsePolicies,
		Rules:     responseRules,
	}

	return &EndpointQueryResponse{[]Endpoint{endpoint}}, nil
}
