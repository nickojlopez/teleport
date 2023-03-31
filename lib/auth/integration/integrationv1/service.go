// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrationv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for
// the Integration gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      services.IntegrationsGetter
	Backend    services.Integrations
	Logger     *logrus.Entry
}

// Service implements the teleport.integration.v1.IntegrationService RPC service.
type Service struct {
	integrationpb.UnimplementedIntegrationServiceServer
	authorizer authz.Authorizer
	cache      services.IntegrationsGetter
	backend    services.Integrations
	logger     *logrus.Entry
}

// NewService returns a new Integrations gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Logger == nil:
		cfg.Logger = logrus.WithField(trace.Component, "integrations.service")
	}

	return &Service{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
	}, nil
}

var _ integrationpb.IntegrationServiceServer = (*Service)(nil)

// ListIntegrations returns a paginated list of all Integration resources.
func (s *Service) ListIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) (*integrationpb.ListIntegrationsResponse, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextKey, err := s.cache.ListIntegrations(ctx, int(req.GetLimit()), req.GetNextKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igs := make([]*types.IntegrationV1, len(results))
	for i, r := range results {
		v1, ok := r.(*types.IntegrationV1)
		if !ok {
			return nil, trace.BadParameter("unexpected Integration type %T", r)
		}
		igs[i] = v1
	}

	return &integrationpb.ListIntegrationsResponse{
		Integrations: igs,
		NextKey:      nextKey,
	}, nil
}

// GetIntegration returns the specified Integration resource.
func (s *Service) GetIntegration(ctx context.Context, req *integrationpb.GetIntegrationRequest) (*types.IntegrationV1, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integration, err := s.cache.GetIntegration(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igV1, ok := integration.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", integration)
	}

	return igV1, nil
}

// CreateIntegration creates a new Okta import rule resource.
func (s *Service) CreateIntegration(ctx context.Context, req *integrationpb.CreateIntegrationRequest) (*types.IntegrationV1, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := s.backend.CreateIntegration(ctx, req.GetIntegration())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", ig)
	}

	return igV1, nil
}

// UpdateIntegration updates an existing Okta import rule resource.
func (s *Service) UpdateIntegration(ctx context.Context, req *integrationpb.UpdateIntegrationRequest) (*types.IntegrationV1, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := s.backend.UpdateIntegration(ctx, req.GetIntegration())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", ig)
	}

	return igV1, nil
}

// DeleteIntegration removes the specified Integration resource.
func (s *Service) DeleteIntegration(ctx context.Context, req *integrationpb.DeleteIntegrationRequest) (*emptypb.Empty, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteIntegration(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllIntegrations removes all Integration resources.
func (s *Service) DeleteAllIntegrations(ctx context.Context, _ *integrationpb.DeleteAllIntegrationsRequest) (*emptypb.Empty, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAllIntegrations(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
