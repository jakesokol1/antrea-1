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
	"github.com/vmware-tanzu/antrea/pkg/apiserver/storage"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	coreinformers "k8s.io/client-go/informers/core/v1"

	antreatypes "github.com/vmware-tanzu/antrea/pkg/controller/types"
)

type EndpointQuerier interface {
	QueryNetworkPolicies(namespace string, podName string) *EndpointQueryResponse
}

// EndpointQueryReplier is responsible for handling query requests from antctl query
type EndpointQueryReplier struct {
	// addressGroupStore is the storage where the populated Address Groups are stored.
	addressGroupStore storage.Interface
	// appliedToGroupStore is the storage where the populated AppliedTo Groups are stored.
	appliedToGroupStore storage.Interface
	// internalNetworkPolicyStore is the storage where the populated internal Network Policy are stored.
	internalNetworkPolicyStore storage.Interface
	// podInformer is used to check existence of a selected pod
	podInformer coreinformers.PodInformer
}

// EndpointQueryResponse is the reply struct for QueryNetworkPolicies
type EndpointQueryResponse struct {
	Endpoints []Endpoint
	Error error
}

// Endpoint holds response information for an endpoint following a query
type Endpoint struct {
	Namespace string
	Name string
	Policies []Policy
	Rules []Rule
}

type PolicyRef struct {
	Namespace string
	Name string
	UID types.UID
}

// Policy holds network policy information to be relayed to client following query endpoint
type Policy struct {
	PolicyRef
	selector metav1.LabelSelector
}

// Rule holds
type Rule struct {
	PolicyRef
	Direction networkingv1beta1.Direction
	RuleIndex int
	Ports []v1.NetworkPolicyPort
}

// NewNetworkPolicyController returns a new *NetworkPolicyController.
func NewEndpointQueryReplier(
	addressGroupStore storage.Interface,
	appliedToGroupStore storage.Interface,
	internalNetworkPolicyStore storage.Interface,
	podInformer coreinformers.PodInformer) *EndpointQueryReplier {
	n := &EndpointQueryReplier{
		addressGroupStore:          addressGroupStore,
		appliedToGroupStore:        appliedToGroupStore,
		internalNetworkPolicyStore: internalNetworkPolicyStore,
		podInformer: podInformer,
	}
	return n
}

//Query functions
func (eq EndpointQueryReplier) QueryNetworkPolicies(namespace string, podName string) *EndpointQueryResponse {
	// check if namespace and podName select an existing pod
	_, err := eq.podInformer.Lister().Pods(namespace).Get(podName)
	// TODO: how to make sure that I handle correct error
	if err != nil {
		return &EndpointQueryResponse{
			Endpoints: nil,
			Error:     err,
		}
	}
	// grab list of all policies from internalNetworkPolicyStore
	internalPolicies := eq.internalNetworkPolicyStore.List()
	// create network policies categories
	applied, _, _ := make([]antreatypes.NetworkPolicy, 0), make([]antreatypes.NetworkPolicy, 0),
		make([]antreatypes.NetworkPolicy, 0)
	// filter all policies into appropriate groups
	for _, policy := range internalPolicies {
		antreaPolicy := policy.(*antreatypes.NetworkPolicy)
		for _, key := range antreaPolicy.AppliedToGroups {
			// Check if policy is applied to endpoint
			//TODO: what is this boolean. what is this error?
			appliedToGroupInterface, _, _ := eq.appliedToGroupStore.Get(key)
			appliedToGroup := appliedToGroupInterface.(*antreatypes.AppliedToGroup)
			// if appliedToGroup selects pod in namespace, append policy to applied category
			for _, podSet := range appliedToGroup.PodsByNode {
				for _, member := range podSet {
					trialPodName, trialNamespace := member.Pod.Name, member.Pod.Namespace
					if podName == trialPodName && namespace == trialNamespace {
						applied = append(applied, *policy.(*antreatypes.NetworkPolicy))
					}
				}
			}
			// Check if policy defines an egress or ingress rule on endpoint
			for _, rule := range policy.(*antreatypes.NetworkPolicy).Rules {
				//TODO: figure out how to see if namespace, pod correlates with NetworkPolicyPeer
				_, _ = rule.From, rule.To
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
	// endpoint
	endpoint := Endpoint{
		Namespace: namespace,
		Name:      podName,
		Policies:  responsePolicies,
		Rules:     responseRules,
	}

	return &EndpointQueryResponse{[]Endpoint{endpoint}, nil}
}