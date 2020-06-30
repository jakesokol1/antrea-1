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
	"github.com/vmware-tanzu/antrea/pkg/apiserver/storage"

	antreatypes "github.com/vmware-tanzu/antrea/pkg/controller/types"
)

type EndpointQuerier interface {
	QueryNetworkPolicies(namespace string, podName string) (applied []antreatypes.NetworkPolicy,
		egress []antreatypes.NetworkPolicy, ingress []antreatypes.NetworkPolicy)
}

// EndpointQueryReplier is responsible for handling query requests from antctl query
type EndpointQueryReplier struct {
	// addressGroupStore is the storage where the populated Address Groups are stored.
	addressGroupStore storage.Interface
	// appliedToGroupStore is the storage where the populated AppliedTo Groups are stored.
	appliedToGroupStore storage.Interface
	// internalNetworkPolicyStore is the storage where the populated internal Network Policy are stored.
	internalNetworkPolicyStore storage.Interface
}


// NewNetworkPolicyController returns a new *NetworkPolicyController.
func NewEndpointQueryReplier(
	addressGroupStore storage.Interface,
	appliedToGroupStore storage.Interface,
	internalNetworkPolicyStore storage.Interface) *EndpointQueryReplier {
	n := &EndpointQueryReplier{
		addressGroupStore:          addressGroupStore,
		appliedToGroupStore:        appliedToGroupStore,
		internalNetworkPolicyStore: internalNetworkPolicyStore,
	}
	return n
}

//Query functions
func (eq EndpointQueryReplier) QueryNetworkPolicies(namespace string, podName string) (applied []antreatypes.NetworkPolicy,
	egress []antreatypes.NetworkPolicy, ingress []antreatypes.NetworkPolicy) {
	// grab list of all policies from internalNetworkPolicyStore
	internalPolicies := eq.internalNetworkPolicyStore.List()
	// create network policies categories
	applied, egress, ingress = make([]antreatypes.NetworkPolicy, 0), make([]antreatypes.NetworkPolicy, 0),
		make([]antreatypes.NetworkPolicy, 0)
	// filter all policies into appropriate groups
	for _, policy := range internalPolicies {
		for _, key := range policy.(*antreatypes.NetworkPolicy).AppliedToGroups {
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

	return
}