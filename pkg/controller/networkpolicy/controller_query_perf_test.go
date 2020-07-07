// +build !race

// Copyright 2020 Antrea Authors
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
	"github.com/google/uuid"
	"github.com/magiconair/properties/assert"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sync"
	"testing"
	"time"
)

/*
TestLargeScaleEndpointQuery tests the execution time and the memory usage of computing a scale
of 100k Namespaces, 100k NetworkPolicies, 100k Pods, where each network policy applies to all pods.
*/
func TestLargeScaleEndpointQuery(t *testing.T) {
	// getObjects taken directly from networkpolicy_controller_perf_test.go
	getObjects := func() ([]*v1.Namespace, []*networkingv1.NetworkPolicy, []*v1.Pod) {
		namespace := rand.String(8)
		namespaces := []*v1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"app": namespace}},
			},
		}
		networkPolicies := []*networkingv1.NetworkPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "np-1", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app-1": "scale-1"}},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"app-1": "scale-1"},
									},
								},
							},
						},
					},
				},
			},
		}
		pods := []*v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod1", UID: types.UID(uuid.New().String()), Labels: map[string]string{"app-1": "scale-1"}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			},
		}
		return namespaces, networkPolicies, pods
	}
	namespaces, networkPolicies, pods := getXObjects(100000, getObjects)
	testQueryEndpoint(t, 30*time.Second, namespaces, networkPolicies, pods)
}

func testQueryEndpoint(t *testing.T, maxExecutionTime time.Duration, namespaces []*v1.Namespace, networkPolicies []*networkingv1.NetworkPolicy, pods []*v1.Pod) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objs := toRunTimeObjects(namespaces, networkPolicies, pods)
	c, querier := makeControllerAndEndpointQueryReplier(objs...)

	c.informerFactory.Start(ctx.Done())
	c.crdInformerFactory.Start(ctx.Done())

	go c.NetworkPolicyController.Run(ctx.Done())

	time.Sleep(15 * time.Second)

	stopCh := make(chan struct{})

	var wg sync.WaitGroup

	// Stat the maximum heap allocation.
	var maxAlloc uint64
	wg.Add(1)
	go func() {
		statMaxMemAlloc(&maxAlloc, 500*time.Millisecond, stopCh)
		wg.Done()
	}()

	// Everything is ready, now start timing.
	start := time.Now()
	// track execution time by calling query endpoint 10 times on some pod
	for i := 0; i < 100; i++ {
		pod, namespace := pods[i].Name, pods[i].Namespace
		response := querier.QueryNetworkPolicies(namespace, pod)
		assert.Equal(t, response.Error, nil)
		assert.Equal(t, len(response.Endpoints[0].Policies), 1)
	}
	// Stop tracking go routines
	stopCh<-struct{}{}
	// Minus the idle time to get the actual execution time.
	executionTime := time.Since(start)
	if executionTime > maxExecutionTime {
		t.Errorf("The actual execution time %v is greater than the maximum value %v", executionTime, maxExecutionTime)
	}

	// Block until all statistics are done.
	wg.Wait()

	t.Logf(`Summary metrics:
NAMESPACES   PODS    NETWORK-POLICIES    TIME(s)    MEMORY(M)    
%-12d %-7d %-19d %-10.2f %-12d 
`, len(namespaces), len(pods), len(networkPolicies), float64(executionTime)/float64(time.Second), maxAlloc/1024/1024)
}



