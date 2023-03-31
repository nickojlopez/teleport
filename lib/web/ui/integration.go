// Copyright 2023 Gravitational, Inc.
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

package ui

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// IntegrationAWSOIDCSpec contain the specific fields for the `aws-oidc` subkind integration.
type IntegrationAWSOIDCSpec struct {
	// RoleARN is the role associated with the integration when SubKind is `aws-oidc`
	RoleARN string `json:"roleARN,omitempty"`
}

// Integration describes the base Integration fields (can be read or wri) Integration
type Integration struct {
	// Name is the Integration name.
	Name string `json:"name"`
	// SubKind is the Integration SubKind.
	SubKind string `json:"subKind"`
	// AWSOIDC contains the fields for `aws-oidc` subkind integration.
	AWSOIDC IntegrationAWSOIDCSpec `json:"awsOIDC"`
}

// IntegrationWithStatus is the representation of a an Integration when reading the resource.
// It contains all the Integration writable fields, plus the read-only ones:
// - status
type IntegrationWithStatus struct {
	Integration
	// Status is the integration status.
	Status string `json:"status"`
}

// CreateIntegrationRequest is a request to create an Integration
type CreateIntegrationRequest struct {
	Integration
}

// CheckAndSetDefaults for the create request.
// Name and SubKind is required.
func (r *CreateIntegrationRequest) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing integration name")
	}

	if r.SubKind == "" {
		return trace.BadParameter("missing subKind")
	}

	return nil
}

// UpdateIntegrationRequest is a request to update an Integration
type UpdateIntegrationRequest struct {
	// AWSOIDC contains the fields for `aws-oidc` subkind integration.
	AWSOIDC *IntegrationAWSOIDCSpec `json:"awsOIDC"`

	// Status contains the status that the integration should be.
	Status *string `json:"status"`

	// IntegrationStatus is the internal representation of the status
	IntegrationStatus types.IntegrationSpecV1_IntegrationStatus `json:"-"`
}

// CheckAndSetDefaults checks if the provided values are valid.
func (r *UpdateIntegrationRequest) CheckAndSetDefaults() error {
	if r.AWSOIDC != nil && r.AWSOIDC.RoleARN == "" {
		return trace.BadParameter("missing awsOIDC.roleARN field")
	}

	if r.Status != nil {
		integrationStatus, ok := types.IntegrationSpecV1_IntegrationStatus_value[*r.Status]
		if !ok {
			return trace.BadParameter("invalid status field")
		}

		r.IntegrationStatus = types.IntegrationSpecV1_IntegrationStatus(integrationStatus)
	}

	if r.AWSOIDC == nil && r.Status == nil {
		return trace.BadParameter("provide at least one field %+v", r)
	}

	return nil
}

// IntegrationsListResponse contains a list of Integrations.
// In case of exceeding the pagination limit (either via query param `limit` or the default 1000)
// a `nextToken` is provided and should be used to obtain the next page (as a query param `startKey`)
type IntegrationsListResponse struct {
	// Items is a list of resources retrieved.
	Items interface{} `json:"items"`
	// NextKey is the position to resume listing events.
	NextKey string `json:"nextKey"`
}

// MakeIntegrations creates a UI list of Integrations.
func MakeIntegrations(igs []types.Integration) []IntegrationWithStatus {
	uiList := make([]IntegrationWithStatus, 0, len(igs))

	for _, ig := range igs {
		uiList = append(uiList, MakeIntegration(ig))
	}

	return uiList
}

// MakeIntegration creates a UI Integration representation.
func MakeIntegration(ig types.Integration) IntegrationWithStatus {
	return IntegrationWithStatus{
		Integration: Integration{
			Name:    ig.GetName(),
			SubKind: ig.GetSubKind(),
			AWSOIDC: IntegrationAWSOIDCSpec{
				RoleARN: ig.GetAWSOIDCIntegrationSpec().RoleARN,
			},
		},
		Status: ig.GetStatus(),
	}
}
