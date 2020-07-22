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

package endpoint

import (
	"github.com/vmware-tanzu/antrea/pkg/antctl/transform"
	"github.com/vmware-tanzu/antrea/pkg/apis/networking/v1beta1"
	"github.com/vmware-tanzu/antrea/pkg/controller/networkpolicy"
	"io"
	"reflect"
	"strconv"
)

type Policy struct {
	Name string
}

type Response struct {
	Namespace    string
	Name         string
	Policies     [][]string
	EgressRules  [][]string
	IngressRules [][]string
}

/*
objectTransform transforms EndpointQueryResponse into a list of Response objects corresponding to unique endpoints.
Though EndpointQueryResponse returns a list of endpoints, using objectTransform rather than listTransform yields a
simpler implementation due to command_definition table printing structure
*/
func objectTransform(o interface{}) (interface{}, error) {
	endpointQueryResponse := o.(*networkpolicy.EndpointQueryResponse)
	responses := make([]*Response, 0)
	// iterate through each endpoint and construct response
	for _, endpoint := range endpointQueryResponse.Endpoints {
		// transform applied policies to string representation
		policies := make([][]string, 0)
		for _, policy := range endpoint.Policies {
			policyStr := []string{policy.Name, policy.Namespace, string(policy.UID)}
			policies = append(policies, policyStr)
		}
		// transform egress and ingress rules to string representation
		egress, ingress := make([][]string, 0), make([][]string, 0)
		for _, rule := range endpoint.Rules {
			ruleStr := []string{rule.Name, rule.Namespace, strconv.Itoa(rule.RuleIndex), string(rule.UID)}
			if rule.Direction == v1beta1.DirectionIn {
				ingress = append(ingress, ruleStr)
			} else if rule.Direction == v1beta1.DirectionOut {
				egress = append(egress, ruleStr)
			} else {
				panic("Unimplemented direction")
			}
		}
		// create full response
		response := &Response{
			Namespace:    endpoint.Namespace,
			Name:         endpoint.Name,
			Policies:     policies,
			EgressRules:  egress,
			IngressRules: ingress,
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func listTransform(l interface{}) (interface{}, error) {
	panic("list transform unimplemented")
}

func Transform(reader io.Reader, single bool) (interface{}, error) {
	return transform.GenericFactory(
		reflect.TypeOf(networkpolicy.EndpointQueryResponse{}),
		reflect.TypeOf([]networkpolicy.EndpointQueryResponse{}),
		objectTransform,
		listTransform,
	)(reader, single)
}

// Note: this pattern of getters for subsections of response follows from transforms of "get" sub command

func (r Response) GetTableLabel() []string {
	return []string{"Endpoint " + r.Namespace + "/" + r.Name}
}

func (r Response) GetPoliciesLabel(exist bool) []string {
	if exist {
		return []string{"Applied Policies:"}
	}
	return []string{"Applied Policies: None"}
}

func (r Response) GetPoliciesHeader() []string {
	return []string{"Name", "Namespace", "UID"}
}

func (r Response) GetEgressLabel(exist bool) []string {
	if exist {
		return []string{"Egress Rules:"}
	}
	return []string{"Egress Rules: None"}
}

func (r Response) GetEgressHeader() []string {
	return []string{"Name", "Namespace", "Index", "UID"}
}

func (r Response) GetIngressLabel(exist bool) []string {
	if exist {
		return []string{"Ingress Rules: "}
	}
	return []string{"Ingress Rules: None"}
}

func (r Response) GetIngressHeader() []string {
	return []string{"Name", "Namespace", "Index", "UID"}
}
