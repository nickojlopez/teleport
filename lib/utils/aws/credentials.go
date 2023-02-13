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

package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

type GetCredentialsRequest struct {
	Provider    client.ConfigProvider
	Expiry      time.Time
	SessionName string
	RoleARN     string
	ExternalID  string
}

// CredentialsGetter defines an interface for obtaining STS credentials.
type CredentialsGetter interface {
	// Get obtains STS credentials.
	Get(ctx context.Context, request GetCredentialsRequest) (*credentials.Credentials, error)
}

type credentialsGettter struct {
}

// NewCredentialsGetter returns a new CredentialsGetter.
func NewCredentialsGetter() CredentialsGetter {
	return &credentialsGettter{}
}

// Get obtains STS credentials.
func (g *credentialsGettter) Get(_ context.Context, request GetCredentialsRequest) (*credentials.Credentials, error) {
	logrus.Debugf("Creating STS session %q.", request.SessionName)
	return stscreds.NewCredentials(request.Provider, request.RoleARN,
		func(cred *stscreds.AssumeRoleProvider) {
			cred.RoleSessionName = request.SessionName
			cred.Expiry.SetExpiration(request.Expiry, 0)

			if request.ExternalID != "" {
				cred.ExternalID = aws.String(request.ExternalID)
			}
		},
	), nil
}

type CachedCredentialsGetterConfig struct {
	Getter   CredentialsGetter
	CacheTTL time.Duration
}

// SetDefaults sets default values for CachedCredentialsGetterConfig.
func (c *CachedCredentialsGetterConfig) SetDefaults() {
	if c.Getter == nil {
		c.Getter = NewCredentialsGetter()
	}
	if c.CacheTTL <= 0 {
		c.CacheTTL = time.Minute
	}
}

type cachedCredentialsGetter struct {
	config CachedCredentialsGetterConfig
	cache  *utils.FnCache
}

func NewCachedCredentialsGetter(config CachedCredentialsGetterConfig) (CredentialsGetter, error) {
	config.SetDefaults()

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: config.CacheTTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cachedCredentialsGetter{
		config: config,
		cache:  cache,
	}, nil
}

// Get returns cached credentials if found, or fetch it from configured getter.
func (g *cachedCredentialsGetter) Get(ctx context.Context, request GetCredentialsRequest) (*credentials.Credentials, error) {
	credentials, err := utils.FnCacheGet(ctx, g.cache, request, func(ctx context.Context) (*credentials.Credentials, error) {
		credentials, err := g.config.Getter.Get(ctx, request)
		return credentials, trace.Wrap(err)
	})
	return credentials, trace.Wrap(err)
}