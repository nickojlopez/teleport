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

package cloud

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/services"
)

func TestAWSSessionCacheKeyBuilder(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		region  string
		roles   []services.AssumeRole
		wantKey string
	}{
		"empty region": {
			// this is sometimes passed when cloud client callers don't care about the region.
			region:  "",
			wantKey: "",
		},
		"only region": {
			region:  "us-west-1",
			wantKey: "us-west-1",
		},
		"with one role": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role", ExternalID: "externalid123"},
			},
			wantKey: "us-west-1:Role[0]:ARN[arn:aws:iam::123456789012:role/test-role]:ExternalID[externalid123]",
		},
		"with multiple roles": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1", ExternalID: "123456"},
				{RoleARN: "arn:aws:iam::222222222222:role/test-role-2", ExternalID: "222222"},
			},
			wantKey: "us-west-1:Role[0]:ARN[arn:aws:iam::123456789012:role/test-role-1]:ExternalID[123456]:Role[1]:ARN[arn:aws:iam::222222222222:role/test-role-2]:ExternalID[222222]",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			builder := NewAWSSessionCacheKeyBuilder(tt.region)
			for _, role := range tt.roles {
				builder.AddRole(role)
			}
			got := builder.String()
			require.Equal(t, tt.wantKey, got)
		})
	}
}

func TestCheckAndSetAssumeRoles(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		region    string // "" maps to default AWS partition: "aws".
		roles     []services.AssumeRole
		wantRoles []services.AssumeRole
		wantErr   string
	}{
		"empty chain": {
			roles:     nil,
			wantRoles: nil,
		},
		"one role in same account": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
			},
			wantRoles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
			},
		},
		"one role in external account": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1", ExternalID: "123456"},
			},
			wantRoles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1", ExternalID: "123456"},
			},
		},
		"roles in correct partition and same account": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-2"},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-3"},
			},
			wantRoles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-2"},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-3"},
			},
		},
		"roles in correct partition and external accounts with external IDs": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "arn:aws:iam::222222222222:role/test-role-2", ExternalID: "222222"},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-3", ExternalID: "333333"},
			},
			wantRoles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "arn:aws:iam::222222222222:role/test-role-2", ExternalID: "222222"},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-3", ExternalID: "333333"},
			},
		},
		"unnecessary external IDs are filtered out": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-2", ExternalID: "222222"},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-3", ExternalID: "333333"},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-4", ExternalID: "333333"},
			},
			wantRoles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-2", ExternalID: ""},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-3", ExternalID: "333333"},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-4", ExternalID: ""},
			},
		},
		"empty role ARNs are filtered out": {
			roles: []services.AssumeRole{
				{RoleARN: "", ExternalID: ""},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1", ExternalID: "123456"},
				{RoleARN: "", ExternalID: "123456"},
				{RoleARN: "arn:aws:iam::222222222222:role/test-role-2", ExternalID: "222222"},
				{RoleARN: "", ExternalID: ""},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-3", ExternalID: "333333"},
				{RoleARN: "", ExternalID: ""},
			},
			wantRoles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1", ExternalID: "123456"},
				{RoleARN: "arn:aws:iam::222222222222:role/test-role-2", ExternalID: "222222"},
				{RoleARN: "arn:aws:iam::333333333333:role/test-role-3", ExternalID: "333333"},
			},
		},
		"role in chain has invalid ARN": {
			region: "us-west-1",
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "foobar"},
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-2"},
			},
			wantErr: "invalid AWS ARN",
		},
		"role in different partition": {
			region: "us-west-1", // maps to "aws" partition.
			roles: []services.AssumeRole{
				// role partition "aws-cn" != "aws"
				{RoleARN: "arn:aws-cn:iam::123456789012:role/test-role-1"},
			},
			wantErr: `expected AWS partition "aws" but got "aws-cn"`,
		},
		"role in external account without external ID": {
			roles: []services.AssumeRole{
				{RoleARN: "arn:aws:iam::123456789012:role/test-role-1"},
				{RoleARN: "arn:aws:iam::222222222222:role/test-role-2"},
			},
			wantErr: "cannot assume external account role",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := checkAndSetAssumeRoles(tt.region, tt.roles)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantRoles, got)
		})
	}
}
