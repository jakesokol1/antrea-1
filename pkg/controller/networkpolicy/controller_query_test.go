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

package networkpolicy

import (
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

// pods represent kubernetes pods for testing proper query results
var pods = []v1.Pod{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podA",
			Namespace: "nsA",
			Labels:    map[string]string{"group": "appliedTo"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name: "container-1",
			}},
			NodeName: "nodeA",
		},
		Status: v1.PodStatus{
			Conditions: []v1.PodCondition{
				{
					Type:   v1.PodReady,
					Status: v1.ConditionTrue,
				},
			},
			PodIP: "1.2.3.4",
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podB",
			Namespace: "nsB",
			Labels:    map[string]string{"group": "appliedTo"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name: "container-1",
			}},
			NodeName: "nodeA",
		},
		Status: v1.PodStatus{
			Conditions: []v1.PodCondition{
				{
					Type:   v1.PodReady,
					Status: v1.ConditionTrue,
				},
			},
			PodIP: "1.2.3.4",
		},
	},

}

// polices represent kubernetes policies for testing proper query results
//
// policy 0: select all pods and deny default ingress
// policy 1: select all pods and deny default egress

var policies = []networkingv1.NetworkPolicy{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-deny-ingress",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: 		"default-deny-egress",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		},
	},
}
// TestInvalidSelector tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) does not
// select any pods
func TestInvalidSelector(t *testing.T) {
	// create controller
	_, controller := newController()
	// create querier with stores inside controller
	address, appliedToGroup, policy := controller.addressGroupStore, controller.appliedToGroupStore, controller.internalNetworkPolicyStore
	endpointQuerier := EndpointQueryReplier{address, appliedToGroup, policy}
	// test appropriate response to QueryNetworkPolices
	namespace, pod := "non-existing-namespace", "non-existing-pod"
	applied, egress, ingress := endpointQuerier.QueryNetworkPolicies(namespace, pod)
	if applied != nil || egress != nil || ingress != nil {
		t.Error("policies should be nil for invalid selector")
	}
}

// TestSingleAppliedPolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects a
// pod which has a single networkpolicy object applied to it
func TestSingleAppliedPolicy(t *testing.T) {
	// create controller
	_, controller := newController()
	// create querier with stores inside controller
	address, appliedToGroup, policy := controller.addressGroupStore, controller.appliedToGroupStore, controller.internalNetworkPolicyStore
	endpointQuerier := EndpointQueryReplier{address, appliedToGroup, policy}
	// add pods and policies
	controller.addPod(&pods[0])
	controller.addPod(&pods[1])
	controller.addNetworkPolicy(&policies[0])
	namespace1, pod1 := "nsA", "podA"
	namespace2, pod2 := "nsB", "podB"

	applied1, _, _ := endpointQuerier.QueryNetworkPolicies(namespace1, pod1)
	applied2, _, _ := endpointQuerier.QueryNetworkPolicies(namespace2, pod2)

	if len(applied1) != 1 || applied1[0].Name != "default-deny-ingress" {
		t.Error("invalid return from querier")
	}

	if len(applied2) != 1 || applied2[0].Name != "default-deny-ingress" {
		t.Error("invalid return from querier")
	}
}

// TestSingleEgressPolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects
// a pod which has a single networkpolicy which defines an egress policy from it.
func TestSingleEgressPolicy(t *testing.T) {
	// create controller
	_, controller := newController()
	// create querier with stores inside controller
	address, appliedToGroup, policy := controller.addressGroupStore, controller.appliedToGroupStore, controller.internalNetworkPolicyStore
	endpointQuerier := EndpointQueryReplier{address, appliedToGroup, policy}
	// add pods and policies
	controller.addPod(&pods[0])
	controller.addPod(&pods[1])
	controller.addNetworkPolicy(&policies[1])
	namespace1, pod1 := "nsA", "podA"
	namespace2, pod2 := "nsB", "podB"
	_, egress1, _ := endpointQuerier.QueryNetworkPolicies(namespace1, pod1)
	_, egress2, _ := endpointQuerier.QueryNetworkPolicies(namespace2, pod2)

	if len(egress1) != 1 || egress1[0].Name != "default-deny-egress" {
		t.Error("invalid return from querier")
	}

	if len(egress2) != 1 || egress2[0].Name != "default-deny-egress" {
		t.Error("invalid return from querier")
	}
}

// TestSingleIngressPolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects
// a pod which has a single networkpolicy which defines an ingress policy from it.
func TestSingleIngressPolicy(t *testing.T) {
	// create controller
	_, controller := newController()
	// create querier with stores inside controller
	address, appliedToGroup, policy := controller.addressGroupStore, controller.appliedToGroupStore, controller.internalNetworkPolicyStore
	endpointQuerier := EndpointQueryReplier{address, appliedToGroup, policy}
	// add pods and policies
	controller.addPod(&pods[0])
	controller.addPod(&pods[1])
	controller.addNetworkPolicy(&policies[1])
	namespace1, pod1 := "nsA", "podA"
	namespace2, pod2 := "nsB", "podB"
	_, egress1, _ := endpointQuerier.QueryNetworkPolicies(namespace1, pod1)
	_, egress2, _ := endpointQuerier.QueryNetworkPolicies(namespace2, pod2)

	if len(egress1) != 1 || egress1[0].Name != "default-deny-ingress" {
		t.Error("invalid return from querier")
	}

	if len(egress2) != 1 || egress2[0].Name != "default-deny-ingress" {
		t.Error("invalid return from querier")
	}
}

// TestMultiplePolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects
// a pod which has multiple networkpolicies which define policies on it.
func TestMultiplePolicy(t *testing.T) {
	// create controller
	_, controller := newController()
	// create querier with stores inside controller
	address, appliedToGroup, policy := controller.addressGroupStore, controller.appliedToGroupStore, controller.internalNetworkPolicyStore
	endpointQuerier := EndpointQueryReplier{address, appliedToGroup, policy}
	// add pods and policies
	controller.addPod(&pods[0])
	controller.addNetworkPolicy(&policies[0])
	controller.addNetworkPolicy(&policies[1])
	namespace1, pod1 := "nsA", "podA"
	applied, _, _ := endpointQuerier.QueryNetworkPolicies(namespace1, pod1)

	if len(applied) != 2 || (applied[0].Name != "default-deny-egress" && applied[0].Name != "default-deny-ingress") ||
		(applied[1].Name != "default-deny-egress" && applied[1].Name != "default-deny-ingress") {
		t.Error("invalid return from querier")
	}
}