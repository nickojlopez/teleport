/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import type * as types from 'teleterm/services/tshd/types';
import type * as uri from 'teleterm/ui/uri';

export class ResourcesService {
  constructor(private tshClient: types.TshClient) {}

  fetchServers(params: types.GetResourcesParams) {
    return this.tshClient.getServers(params);
  }

  // TODO(ravicious): Refactor it to use logic similar to that in the Web UI.
  // https://github.com/gravitational/teleport/blob/2a2b08dbfdaf71706a5af3812d3a7ec843d099b4/lib/web/apiserver.go#L2471
  async getServerByHostname(
    clusterUri: uri.ClusterUri,
    hostname: string
  ): Promise<types.Server | undefined> {
    const query = `name == "${hostname}"`;
    const { agentsList: servers } = await this.fetchServers({
      clusterUri,
      query,
      limit: 2,
      sort: null,
    });

    if (servers.length > 1) {
      throw new AmbiguousHostnameError(hostname);
    }

    return servers[0];
  }

  fetchDatabases(params: types.GetResourcesParams) {
    return this.tshClient.getDatabases(params);
  }

  fetchKubes(params: types.GetResourcesParams) {
    return this.tshClient.getKubes(params);
  }

  async getDbUsers(dbUri: uri.DatabaseUri): Promise<string[]> {
    return await this.tshClient.listDatabaseUsers(dbUri);
  }

  /**
   * searchResources searches for the given list of space-separated keywords across all resource
   * types on the given cluster.
   *
   * It does so by issuing a separate request for each resource type. It fails if any of those
   * requests fail.
   *
   * The results need to be wrapped in SearchResult because if we returned raw types (Server,
   * Database, Kube) then there would be no easy way to differentiate between them on type level.
   */
  async searchResources(
    clusterUri: uri.ClusterUri,
    search: string
  ): Promise<SearchResult[]> {
    const params = { search, clusterUri, sort: null, limit: 100 };

    const servers = this.fetchServers(params).then(res =>
      res.agentsList.map(resource => ({
        kind: 'server' as const,
        resource,
        labelMatches: [],
        resourceMatches: [],
        score: 0,
      }))
    );
    const databases = this.fetchDatabases(params).then(res =>
      res.agentsList.map(resource => ({
        kind: 'database' as const,
        resource,
        labelMatches: [],
        resourceMatches: [],
        score: 0,
      }))
    );
    const kubes = this.fetchKubes(params).then(res =>
      res.agentsList.map(resource => ({
        kind: 'kube' as const,
        resource,
        labelMatches: [],
        resourceMatches: [],
        score: 0,
      }))
    );

    return (await Promise.all([servers, databases, kubes])).flat();
  }
}

export class AmbiguousHostnameError extends Error {
  constructor(hostname: string) {
    super(`Ambiguous hostname "${hostname}"`);
    this.name = 'AmbiguousHostname';
  }
}

type SearchResultBase<Kind, Resource> = {
  kind: Kind;
  resource: Resource;
  labelMatches: LabelMatch[];
  resourceMatches: ResourceMatch<Resource>[];
  score: number;
};

export type SearchResultServer = SearchResultBase<'server', types.Server>;
export type SearchResultDatabase = SearchResultBase<'database', types.Database>;
export type SearchResultKube = SearchResultBase<'kube', types.Kube>;

export type SearchResult =
  | SearchResultServer
  | SearchResultDatabase
  | SearchResultKube;

export type LabelMatch = {
  kind: 'label-name' | 'label-value';
  labelName: string;
  searchTerm: string;
};

// TODO: Limit <Resource> to only searchable resources.
// type SearchableResources = types.Server | types.Database | types.Kube;
export type ResourceMatch<Resource> = {
  // TODO: Limit this to only searchable fields.
  field: keyof Resource;
  searchTerm: string;
};
