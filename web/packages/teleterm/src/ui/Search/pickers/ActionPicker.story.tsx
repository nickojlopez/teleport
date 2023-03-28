import React from 'react';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { routing } from 'teleterm/ui/uri';

import { ResourceSearchResult } from '../searchResult';
import {
  makeDatabase,
  makeKube,
  makeResult,
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
  const searchResults = [
    makeResult({
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
    makeResult({
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
    makeResult({
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
    makeResult({
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
    makeResult({
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
    makeResult({
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
    makeResult({
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
    makeResult({
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
    makeResult({
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
  ];
  const attempt = makeSuccessAttempt(searchResults);

  return (
    <div
      css={`
        width: 600px;
      `}
    >
      <ResultList<ResourceSearchResult>
        attempts={[attempt]}
        onPick={() => {}}
        onBack={() => {}}
        render={searchResult => {
          const Component = ComponentMap[searchResult.kind];

          return {
            key: searchResult.resource.uri,
            Component: (
              <Component
                searchResult={searchResult}
                getClusterName={routing.parseClusterName}
              />
            ),
          };
        }}
      />
    </div>
  );
};
