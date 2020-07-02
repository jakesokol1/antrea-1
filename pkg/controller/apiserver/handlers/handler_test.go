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

package handlers

import (
	"encoding/json"
	"github.com/vmware-tanzu/antrea/pkg/controller/networkpolicy"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/antrea/pkg/controller/apiserver/handlers/endpoint"
	queriermock "github.com/vmware-tanzu/antrea/pkg/controller/networkpolicy/testing"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestCase struct {
	// query arguments sent to handler function
	handlerRequest  string
	expectedStatus  int
	// expected result written by handler function
	expectedContent *networkpolicy.EndpointQueryResponse

	// arguments of call to mock
	argsMock       []string
	// results of call to mock
	mockQueryResponse    *networkpolicy.EndpointQueryResponse
}

var responses = []*networkpolicy.EndpointQueryResponse{
	{
		Endpoints: nil,
		Error:     errors.NewNotFound(v1.Resource("pod"), "pod"),
	},
	{
		Endpoints: []networkpolicy.Endpoint{
			{
				Policies: []networkpolicy.Policy{
					{
						PolicyRef: networkpolicy.PolicyRef{Name: "policy1"},
					},
				},
			},
		},
		Error: nil,
	},
	{
		Endpoints: []networkpolicy.Endpoint{
			{
				Policies: []networkpolicy.Policy{
					{
						PolicyRef: networkpolicy.PolicyRef{Name: "policy1"},
					},
					{
						PolicyRef: networkpolicy.PolicyRef{Name: "policy2"},
					},
				},
			},
		},
		Error: nil,
	},
}

// TestIncompleteArguments tests how the handler function responds when the user passes in a query command
// with incomplete arguments (for now, missing pod or namespace)
func TestIncompleteArguments(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	// sample selector arguments (right now, only supports podname and namespace)
	pod, namespace := "pod", "namespace"
	// outline test cases with expected behavior
	testCases := map[string]TestCase{
		"Responds with error given no name and no namespace": {
			handlerRequest:           "",
			expectedStatus:  http.StatusBadRequest,
			argsMock: []string{"", ""},
		},
		"Responds with error given no name": {
			handlerRequest:           "?namespace=namespace",
			expectedStatus:  http.StatusBadRequest,
			argsMock: []string{namespace, ""},
		},
		"Responds with error given no namespace": {
			handlerRequest:           "?pod=pod",
			expectedStatus:  http.StatusBadRequest,
			argsMock: []string{"", pod},
		},
	}

	evaluateTestCases(testCases, mockCtrl, t)

}

//TODO: need to differentiate the return from invalid and incomplete arguments to give correct instructions to user

// TestInvalidArguments tests how the handler function responds when the user passes in a selector which does not select
// any existing endpoint
func TestInvalidArguments(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	// sample selector arguments (right now, only supports podname and namespace)
	pod, namespace := "pod", "namespace"
	// outline test cases with expected behavior
	testCases := map[string]TestCase{
		"Responds with error given no invalid selection": {
			handlerRequest:           "?namespace=namespace&pod=pod",
			expectedStatus:  http.StatusNotFound,
			argsMock: []string{namespace, pod},
			mockQueryResponse: &networkpolicy.EndpointQueryResponse{
				Endpoints: nil,
				Error:     errors.NewNotFound(v1.Resource("pod"), "pod"),
			},
		},
	}

	evaluateTestCases(testCases, mockCtrl, t)

}


func TestSinglePolicyResponse(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	// sample selector arguments (right now, only supports podName and namespace)
	pod, namespace := "pod", "namespace"
	// outline test cases with expected behavior
	testCases := map[string]TestCase{
		"Responds with list of single element": {
			handlerRequest:           "?namespace=namespace&pod=pod",
			expectedStatus:  http.StatusOK,
			expectedContent: responses[1],
			argsMock: []string{namespace, pod},
			mockQueryResponse: &networkpolicy.EndpointQueryResponse{Endpoints: []networkpolicy.Endpoint{
				{
					Policies: []networkpolicy.Policy{
						{
							PolicyRef: networkpolicy.PolicyRef{Name: "policy1"},
						},
					},
				},
			},
			},
		},
	}

	evaluateTestCases(testCases, mockCtrl, t)

}

func TestMultiPolicyResponse(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	// sample selector arguments (right now, only supports podName and namespace)
	pod, namespace := "pod", "namespace"
	// outline test cases with expected behavior
	testCases := map[string]TestCase{
		"Responds with list of single element": {
			handlerRequest:           "?namespace=namespace&pod=pod",
			expectedStatus:  http.StatusOK,
			expectedContent: responses[2],
			argsMock: []string{namespace, pod},
			mockQueryResponse: &networkpolicy.EndpointQueryResponse{Endpoints: []networkpolicy.Endpoint{
				{
					Policies: []networkpolicy.Policy{
						{
							PolicyRef: networkpolicy.PolicyRef{Name: "policy1"},
						},
						{
							PolicyRef: networkpolicy.PolicyRef{Name: "policy2"},
						},
					},
				},
			}},
		},
	}

	evaluateTestCases(testCases, mockCtrl, t)

}

func evaluateTestCases(testCases map[string]TestCase, mockCtrl *gomock.Controller, t *testing.T) {
	for _, tc := range testCases {
		// create mock querier with expected behavior outlined in testCase
		mockQuerier := queriermock.NewMockEndpointQuerier(mockCtrl)
		if tc.expectedStatus != http.StatusBadRequest {
			mockQuerier.EXPECT().QueryNetworkPolicies(tc.argsMock[0], tc.argsMock[1]).Return(tc.mockQueryResponse)
		}
		// initialize handler with mockQuerier
		handler := endpoint.HandleFunc(mockQuerier)
		// create http using handlerArgs and serve the http request
		req, err := http.NewRequest(http.MethodGet, tc.handlerRequest, nil)
		assert.Nil(t, err)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		assert.Equal(t, tc.expectedStatus, recorder.Code)
		if tc.expectedStatus != http.StatusOK {
			return
		}
		// check response is expected
		var received networkpolicy.EndpointQueryResponse
		err = json.Unmarshal(recorder.Body.Bytes(), &received)
		assert.Nil(t, err)
		for i, policy := range tc.expectedContent.Endpoints[0].Policies {
			assert.Equal(t, policy.Name, received.Endpoints[0].Policies[i].Name)
		}
	}
}