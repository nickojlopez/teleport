/*
Copyright 2022 Gravitational, Inc.

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

package db

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

var (
	wildcardLabels = map[string]string{types.Wildcard: types.Wildcard}
	envProdLabels  = map[string]string{"env": "prod"}
	envDevLabels   = map[string]string{"env": "dev"}
)

func toTypeLabels(labels map[string]string) types.Labels {
	result := make(types.Labels)
	for key, value := range labels {
		result[key] = utils.Strings{value}
	}
	return result
}

func mustMakeAWSFetchers(t *testing.T, clients cloud.AWSClients, matchers []services.AWSMatcher) []common.Fetcher {
	t.Helper()

	fetchers, err := MakeAWSFetchers(context.Background(), clients, matchers)
	require.NoError(t, err)
	require.NotEmpty(t, fetchers)

	for _, fetcher := range fetchers {
		require.Equal(t, types.KindDatabase, fetcher.ResourceType())
		require.Equal(t, types.CloudAWS, fetcher.Cloud())
	}
	return fetchers
}

func mustMakeAWSFetchersForMatcher(t *testing.T, clients *cloud.TestCloudClients, matcherType, region string, tags types.Labels) []common.Fetcher {
	t.Helper()
	matchers := []services.AWSMatcher{{
		Types:      []string{matcherType},
		Regions:    []string{region},
		Tags:       tags,
		AssumeRole: testAssumeRole,
	}}
	mustAddAssumedRolesAndMockSessionsForMatchers(t, matchers, clients)
	return mustMakeAWSFetchers(t, clients, matchers)
}

func mustMakeAzureFetchers(t *testing.T, clients cloud.AzureClients, matchers []services.AzureMatcher) []common.Fetcher {
	t.Helper()

	fetchers, err := MakeAzureFetchers(clients, matchers)
	require.NoError(t, err)
	require.NotEmpty(t, fetchers)

	for _, fetcher := range fetchers {
		require.Equal(t, types.KindDatabase, fetcher.ResourceType())
		require.Equal(t, types.CloudAzure, fetcher.Cloud())
	}
	return fetchers
}

func mustGetDatabases(t *testing.T, fetchers []common.Fetcher) types.Databases {
	t.Helper()

	var all types.Databases
	for _, fetcher := range fetchers {
		resources, err := fetcher.Get(context.TODO())
		require.NoError(t, err)

		databases, err := resources.AsDatabases()
		require.NoError(t, err)

		all = append(all, databases...)
	}
	return all
}

// testAssumeRole is a fixture for testing fetchers.
// every matcher, stub database, and mock AWS Session created uses this fixture.
// Tests will cover:
//   - that fetchers use the configured assume role when using AWS cloud clients.
//   - that databases discovered and created by fetchers have the assumed role used to discover them populated.
var testAssumeRole = services.AssumeRole{
	RoleARN:    "arn:aws:iam::123456789012:role/test-role",
	ExternalID: "externalID123",
}

// mustAddAssumedRolesAndMockSessionsForMatchers injects a test assume role and injects
// mock AWS sessions for the assumed role into test cloud clients.
func mustAddAssumedRolesAndMockSessionsForMatchers(t *testing.T, matchers []services.AWSMatcher, clients *cloud.TestCloudClients) {
	t.Helper()
	// configure all the test matchers to have an assumed role and inject mock AWS sessions into cloud clients for them.
	awsSessions := make(map[string]*session.Session)
	for i := range matchers {
		matchers[i].AssumeRole = testAssumeRole
		for _, region := range matchers[i].Regions {
			cacheKey, session := makeAWSSession(t, region, testAssumeRole)
			awsSessions[cacheKey] = session
		}
	}
	clients.AWSAssumeRoleSessions = awsSessions
}

// makeAWSSession is a test helper to build a mock cached aws session for a given assume role chain.
func makeAWSSession(t *testing.T, region string, roles ...services.AssumeRole) (string, *session.Session) {
	t.Helper()
	awsSession, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
			AccessKeyID:     "fakeClientKeyID",
			SecretAccessKey: "fakeClientSecret",
		}}),
		Region: aws.String(region),
	})
	require.NoError(t, err)
	keyBuilder := cloud.NewAWSSessionCacheKeyBuilder(region)
	for i := range roles {
		keyBuilder.AddRole(roles[i])
	}
	return keyBuilder.String(), awsSession
}
