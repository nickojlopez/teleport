import React from 'react';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { routing } from 'teleterm/ui/uri';

import { SearchResult } from '../searchResult';
import {
  makeDatabase,
  makeKube,
  makeResourceResult,
  makeServer,
  makeLabelsList,
} from '../searchResultTestHelpers';

import { ComponentMap } from './ActionPicker';
import { ResultList } from './ResultList';

import type * as uri from 'teleterm/ui/uri';

export default {
  title: 'Teleterm/Search/ActionPicker',
};

const clusterUri: uri.ClusterUri = '/clusters/teleport-local';

export const Items = () => {
  return (
    <div
      css={`
        max-width: 600px;
      `}
    >
      <List />
    </div>
  );
};
export const ItemsNarrow = () => {
  return (
    <div
      css={`
        max-width: 300px;
      `}
    >
      <List />
    </div>
  );
};

const List = () => {
  const searchResults: SearchResult[] = [
    makeResourceResult({
      kind: 'server',
      resource: makeServer({
        hostname: 'long-label-list',
        uri: `${clusterUri}/servers/2f96e498-88ec-442f-a25b-569fa915041c`,
        name: '2f96e498-88ec-442f-a25b-569fa915041c',
        labelsList: makeLabelsList({
          arch: 'aarch64',
          external: '32.192.113.93',
          internal: '10.0.0.175',
          kernel: '5.13.0-1234-aws',
          service: 'ansible',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'server',
      resource: makeServer({
        hostname: 'short-label-list',
        addr: '',
        tunnel: true,
        uri: `${clusterUri}/servers/90a29595-aac7-42eb-a484-c6c0e23f1a21`,
        name: '90a29595-aac7-42eb-a484-c6c0e23f1a21',
        labelsList: makeLabelsList({
          arch: 'aarch64',
          service: 'ansible',
          external: '32.192.113.93',
          internal: '10.0.0.175',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'server',
      resourceMatches: [{ field: 'name', searchTerm: 'bbaaceba-6bd1-4750' }],
      resource: makeServer({
        hostname: 'uuid-match',
        addr: '',
        tunnel: true,
        uri: `${clusterUri}/servers/bbaaceba-6bd1-4750-9d3d-1a80e0cc8a63`,
        name: 'bbaaceba-6bd1-4750-9d3d-1a80e0cc8a63',
        labelsList: makeLabelsList({
          internal: '10.0.0.175',
          service: 'ansible',
          external: '32.192.113.93',
          arch: 'aarch64',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/no-desc`,
        name: 'no-desc',
        desc: '',
        labelsList: makeLabelsList({
          'aws/Accounting': 'dev-ops',
          'aws/Environment': 'demo-13-biz',
          'aws/Name': 'db-bastion-4-13biz',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          engine: '🐘',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/short-desc`,
        name: 'short-desc',
        desc: 'Lorem ipsum',
        labelsList: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Name': 'db-bastion-4-13biz',
          'aws/Accounting': 'dev-ops',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          engine: '🐘',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/long-desc`,
        name: 'long-desc',
        desc: 'Eget dignissim lectus nisi vitae nunc',
        labelsList: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Name': 'db-bastion-4-13biz',
          'aws/Accounting': 'dev-ops',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          engine: '🐘',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/super-long-desc`,
        name: 'super-long-desc',
        desc: 'Duis id tortor at purus tincidunt finibus. Mauris eu semper orci, non commodo lacus. Praesent sollicitudin magna id laoreet porta. Nunc lobortis varius sem vel fringilla.',
        labelsList: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Accounting': 'dev-ops',
          'aws/Name': 'db-bastion-4-13biz',
          engine: '🐘',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'kube',
      resource: makeKube({
        name: 'short-label-list',
        labelsList: makeLabelsList({
          'im-just-a-smol': 'kube',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'kube',
      resource: makeKube({
        name: 'long-label-list',
        uri: `${clusterUri}/kubes/long-label-list`,
        labelsList: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          'aws/Name': 'db-bastion-4-13biz',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    {
      kind: 'resource-type-filter',
      resource: 'kubes',
      nameMatch: '',
      score: 0,
    },
    {
      kind: 'cluster-filter',
      resource: {
        name: 'teleport-local',
        uri: clusterUri,
        authClusterId: '',
        connected: true,
        leaf: false,
        proxyHost: 'teleport-local.dev:3090',
      },
      nameMatch: '',
      score: 0,
    },
  ];
  const attempt = makeSuccessAttempt(searchResults);

  return (
    <ResultList<SearchResult>
      attempts={[attempt]}
      onPick={() => {}}
      onBack={() => {}}
      render={searchResult => {
        const Component = ComponentMap[searchResult.kind];

        return {
          key:
            searchResult.kind !== 'resource-type-filter'
              ? searchResult.resource.uri
              : searchResult.resource,
          Component: (
            <Component
              searchResult={searchResult}
              getClusterName={routing.parseClusterName}
            />
          ),
        };
      }}
    />
  );
};
