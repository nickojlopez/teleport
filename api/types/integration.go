/*
Copyright 2023 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

const (
	// IntegrationSubKindAWSOIDC is an integration with AWS that uses OpenID Connect as an Identity Provider.
	// More information can be found here: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html
	// This Integration requires the RoleARN Spec field to be present.
	// That is the AWS role to be used when creating a token, to then issue an API Call to AWS.
	IntegrationSubKindAWSOIDC = "aws-oidc"
)

// Integration specifies is a connection configuration between Teleport and a 3rd party system.
type Integration interface {
	ResourceWithLabels

	// GetStatusCode returns the current integration status as a code.
	GetStatusCode() IntegrationSpecV1_IntegrationStatus

	// GetStatus returns the integration status.
	GetStatus() string

	// SetStatusCode sets the integration status.
	SetStatusCode(IntegrationSpecV1_IntegrationStatus)

	// Fields for `aws-oidc` spec

	// GetAWSOIDCIntegrationSpec returns the `aws-oidc` specific spec fields.
	GetAWSOIDCIntegrationSpec() *AWSOIDCIntegrationSpecV1
	// SetAWSOIDCIntegrationSpec sets the `aws-oidc` specific spec fields.
	SetAWSOIDCIntegrationSpec(*AWSOIDCIntegrationSpecV1)
}

var _ ResourceWithLabels = (*IntegrationV1)(nil)

// NewIntegrationAWSOIDC returns a new `aws-oidc` subkind Integration
func NewIntegrationAWSOIDC(md Metadata, spec *AWSOIDCIntegrationSpecV1) (*IntegrationV1, error) {
	ig := &IntegrationV1{
		ResourceHeader: ResourceHeader{
			Metadata: md,
			Kind:     KindIntegration,
			Version:  V1,
			SubKind:  IntegrationSubKindAWSOIDC,
		},
		Spec: IntegrationSpecV1{
			SubKindSpec: &IntegrationSpecV1_AWSOIDC{
				AWSOIDC: spec,
			},
			Status: IntegrationSpecV1_INTEGRATION_STATUS_PAUSED,
		},
	}
	if err := ig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return ig, nil
}

// String returns the integration string representation.
func (ig *IntegrationV1) String() string {
	return fmt.Sprintf("IntegrationV1(Name=%v, SubKind=%s, Labels=%v)",
		ig.GetName(), ig.GetSubKind(), ig.GetAllLabels())
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (ig *IntegrationV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(ig.GetAllLabels()), ig.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (ig *IntegrationV1) setStaticFields() {
	ig.Kind = KindIntegration
	ig.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (ig *IntegrationV1) CheckAndSetDefaults() error {
	ig.setStaticFields()
	if err := ig.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(ig.Spec.CheckAndSetDefaults())
}

// CheckAndSetDefaults validates and sets default values for a integration.
func (s *IntegrationSpecV1) CheckAndSetDefaults() error {
	if s.SubKindSpec == nil {
		return trace.BadParameter("invalid subKindSpec")
	}

	switch integrationSubKind := s.SubKindSpec.(type) {
	case *IntegrationSpecV1_AWSOIDC:
		err := integrationSubKind.CheckAndSetDefaults()
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown integration subkind: %T", integrationSubKind)
	}

	return nil
}

// CheckAndSetDefaults validates an agent mesh integration.
func (s *IntegrationSpecV1_AWSOIDC) CheckAndSetDefaults() error {
	if s == nil || s.AWSOIDC == nil {
		return trace.BadParameter("awsoidc is required for %q subkind", IntegrationSubKindAWSOIDC)
	}

	if s.AWSOIDC.RoleARN == "" {
		return trace.BadParameter("roleARN is required for %q subkind", IntegrationSubKindAWSOIDC)
	}

	return nil
}

// GetAWSOIDCIntegrationSpec returns the specific spec fields for `aws-oidc` subkind integrations.
func (ig *IntegrationV1) GetAWSOIDCIntegrationSpec() *AWSOIDCIntegrationSpecV1 {
	return ig.Spec.GetAWSOIDC()
}

// SetAWSOIDCIntegrationSpec sets the specific fields for the `aws-oidc` subkind integration.
func (ig *IntegrationV1) SetAWSOIDCIntegrationSpec(awsOIDCSpec *AWSOIDCIntegrationSpecV1) {
	ig.Spec.SubKindSpec = &IntegrationSpecV1_AWSOIDC{
		AWSOIDC: awsOIDCSpec,
	}
}

// GetStatusCode returns the integration status as a code.
// It can be one of:
// - paused: integration was just configured or was disabled by the user
// - running: integration is ready to do requests
// - error: integration has an error and should
func (ig *IntegrationV1) GetStatusCode() IntegrationSpecV1_IntegrationStatus {
	return ig.Spec.Status
}

// GetStatus returns the integration status.
// It can be one of:
// - paused: integration was just configured or was disabled by the user
// - running: integration is ready to do requests
// - error: integration has an error and should
func (ig *IntegrationV1) GetStatus() string {
	return ig.Spec.Status.String()
}

// SetStatusCode sets the integration status.
func (ig *IntegrationV1) SetStatusCode(st IntegrationSpecV1_IntegrationStatus) {
	ig.Spec.Status = st
}

// Integrations is a list of Integration resources.
type Integrations []Integration

// AsResources returns these groups as resources with labels.
func (igs Integrations) AsResources() []ResourceWithLabels {
	resources := make([]ResourceWithLabels, len(igs))
	for i, ig := range igs {
		resources[i] = ig
	}
	return resources
}

// Len returns the slice length.
func (igs Integrations) Len() int { return len(igs) }

// Less compares integrations by name.
func (igs Integrations) Less(i, j int) bool { return igs[i].GetName() < igs[j].GetName() }

// Swap swaps two integrations.
func (igs Integrations) Swap(i, j int) { igs[i], igs[j] = igs[j], igs[i] }

// UnmarshalJSON is a custom UnmarshalJSON because of the oneof field.
// oneof fields from protobuf convert into a go interface.
// When trying to Unmarshal the SubKindSpec field, it errors out because it is an interface
// and go can't guess which struct to use.
//
// The solution is to look to the SubKind field and instantiate the proper field.
// The standard json.Unmarshal is used afterwards.
func (ig *IntegrationV1) UnmarshalJSON(data []byte) error {
	if ig == nil {
		ig = &IntegrationV1{}
	}

	type rootFields struct {
		ResourceHeader `json:""`
		Spec           json.RawMessage `json:"spec"`
	}

	rf := rootFields{}

	err := json.Unmarshal(data, &rf)
	if err != nil {
		return trace.Wrap(err)
	}

	ig.ResourceHeader = rf.ResourceHeader

	switch ig.ResourceHeader.SubKind {
	case IntegrationSubKindAWSOIDC:
		ig.Spec.SubKindSpec = &IntegrationSpecV1_AWSOIDC{}
	default:
		return trace.BadParameter("invalid subkind %q", ig.ResourceHeader.SubKind)
	}

	if err := json.Unmarshal(rf.Spec, &ig.Spec); err != nil {
		return trace.Wrap(err)
	}

	if err := ig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
