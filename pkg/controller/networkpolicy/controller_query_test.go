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
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
	"time"
)

// pods represent kubernetes pods for testing proper query results
var pods = []v1.Pod{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podA",
			Namespace: "testNamespace",
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
			Namespace: "testNamespace",
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
			Name: "default-deny-egress",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		},
	},
}

var namespaces = []v1.Namespace{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNamespace",
			UID:  "testNamespaceUID",
		},
	},
}

func makeControllerAndEndpointQueryReplier(objects ...runtime.Object) (*networkPolicyController, *EndpointQueryReplier) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// create controller
	_, controller := newController(objects...)
	// create querier with stores inside controller
	querier := NewEndpointQueryReplier(controller.NetworkPolicyController)
	// start informers and run controller
	controller.informerFactory.Start(ctx.Done())
	controller.crdInformerFactory.Start(ctx.Done())
	go controller.NetworkPolicyController.Run(ctx.Done())
	// TODO: replace this with logic which waits for the networkpolicy controller to initialize (look into perf testing)
	time.Sleep(4 * time.Second)
	return controller, querier
}

// TestInvalidSelector tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) does not
// select any pods
func TestInvalidSelector(t *testing.T) {
	_, endpointQuerier := makeControllerAndEndpointQueryReplier()
	// test appropriate response to QueryNetworkPolices
	namespace, pod := "non-existing-namespace", "non-existing-pod"
	_, err := endpointQuerier.QueryNetworkPolicies(namespace, pod)

	assert.Equal(t, errors.NewNotFound(v1.Resource("pod"), pod), err, "expected not found error")
}

// TestSingleAppliedPolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects a
// pod which has a single networkpolicy object applied to it
func TestSingleAppliedPolicy(t *testing.T) {
	_, endpointQuerier := makeControllerAndEndpointQueryReplier(&namespaces[0], &pods[0], &policies[0])
	namespace1, pod1 := "testNamespace", "podA"
	response1, err := endpointQuerier.QueryNetworkPolicies(namespace1, pod1)
	require.Equal(t, nil, err)
	assert.Equal(t, response1.Endpoints[0].Policies[0].PolicyRef.Name, "default-deny-ingress")
}

// TestSingleEgressPolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects
// a pod which has a single networkpolicy which defines an egress policy from it.
func TestSingleEgressPolicy(t *testing.T) {
	assert.Fail(t, "unimplemented")
}

// TestSingleIngressPolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects
// a pod which has a single networkpolicy which defines an ingress policy from it.
func TestSingleIngressPolicy(t *testing.T) {
	assert.Fail(t, "unimplemented")
}

// TestMultiplePolicy tests the result of QueryNetworkPolicy when the selector (right now pod, namespace) selects
// a pod which has multiple networkpolicies which define policies on it.
func TestMultiplePolicy(t *testing.T) {
	_, endpointQuerier := makeControllerAndEndpointQueryReplier(&namespaces[0], &pods[0], &policies[0], &policies[1])
	namespace1, pod1 := "testNamespace", "podA"
	response, err := endpointQuerier.QueryNetworkPolicies(namespace1, pod1)
	require.Equal(t, nil, err)
	assert.True(t, response.Endpoints[0].Policies[0].Name == "default-deny-egress" ||
		response.Endpoints[0].Policies[0].Name == "default-deny-ingress")
	assert.True(t, response.Endpoints[0].Policies[1].Name == "default-deny-egress" ||
		response.Endpoints[0].Policies[1].Name == "default-deny-ingress")
}
