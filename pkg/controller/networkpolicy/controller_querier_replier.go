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
		}, nil
	}
	type ruleTemp struct {
		policy *antreatypes.NetworkPolicy
		index int
	}
	// create network policies categories
	applied, ingress, egress := make([]*antreatypes.NetworkPolicy, 0), make([]*ruleTemp, 0),
		make([]*ruleTemp, 0)
	// get all appliedToGroups using pod index, then get applied policies using appliedToGroup
	appliedToGroups, err := eq.networkPolicyController.appliedToGroupStore.GetByIndex(store.PodIndex, podName + "/" + namespace)
	if err != nil {
		return nil, err
	}
	for _, appliedToGroup := range appliedToGroups {
		policies, err := eq.networkPolicyController.internalNetworkPolicyStore.GetByIndex(store.AppliedToGroupIndex,
			string(appliedToGroup.(*antreatypes.AppliedToGroup).UID))
		if err != nil {
			return nil, err
		}
		for _, policy := range policies {
			applied = append(applied, policy.(*antreatypes.NetworkPolicy))
		}
	}
	// get all addressGroups using pod index, then get ingress and egress policies using addressGroup
	addressGroups, err := eq.networkPolicyController.addressGroupStore.GetByIndex(store.PodIndex, podName + "/" + namespace)
	if err != nil {
		return nil, err
	}
	for _, addressGroup := range addressGroups {
		policies, err := eq.networkPolicyController.internalNetworkPolicyStore.GetByIndex(store.AddressGroupIndex,
			string(addressGroup.(*antreatypes.AddressGroup).UID))
		if err != nil {
			return nil, err
		}
		for _, policy := range policies {
			for i, rule := range policy.(*antreatypes.NetworkPolicy).Rules {
				for _, addressGroupTrial := range rule.To.AddressGroups {
					if addressGroupTrial == string(addressGroup.(*antreatypes.AddressGroup).UID) {
						egress = append(egress, &ruleTemp{policy: policy.(*antreatypes.NetworkPolicy), index: i})
					}
				}
				for _, addressGroupTrial := range rule.From.AddressGroups {
					if addressGroupTrial == string(addressGroup.(*antreatypes.AddressGroup).UID) {
						ingress = append(ingress, &ruleTemp{policy: policy.(*antreatypes.NetworkPolicy), index: i})
					}
				}
			}
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
	// create rules based on egress and ingress policies
	for _, internalPolicy := range egress {
		newRule := Rule{
			PolicyRef: PolicyRef{
				Namespace: internalPolicy.policy.Namespace,
				Name:      internalPolicy.policy.Name,
				UID:       internalPolicy.policy.UID,
			},
			Direction: networkingv1beta1.DirectionOut,
			RuleIndex: internalPolicy.index,
		}
		responseRules = append(responseRules, newRule)
	}
	for _, internalPolicy := range ingress {
		newRule := Rule{
			PolicyRef: PolicyRef{
				Namespace: internalPolicy.policy.Namespace,
				Name:      internalPolicy.policy.Name,
				UID:       internalPolicy.policy.UID,
			},
			Direction: networkingv1beta1.DirectionIn,
			RuleIndex: internalPolicy.index,
		}
		responseRules = append(responseRules, newRule)
	}
	// endpoint
	endpoint := Endpoint{
		Namespace: namespace,
		Name:      podName,
		Policies:  responsePolicies,
		Rules:     responseRules,
	}

	return &EndpointQueryResponse{[]Endpoint{endpoint}}, nil
}
