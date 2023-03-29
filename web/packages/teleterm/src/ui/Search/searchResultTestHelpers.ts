import { ResourceSearchResult } from './searchResult';

import type * as tsh from 'teleterm/services/tshd/types';

export const makeServer = (props: Partial<tsh.Server>): tsh.Server => ({
  uri: '/clusters/teleport-local/servers/178ef081-259b-4aa5-a018-449b5ea7e694',
  tunnel: false,
  name: '178ef081-259b-4aa5-a018-449b5ea7e694',
  hostname: 'foo',
  addr: '127.0.0.1:3022',
  labelsList: [],
  ...props,
});

export const makeDatabase = (props: Partial<tsh.Database>): tsh.Database => ({
  uri: '/clusters/teleport-local/dbs/foo',
  name: 'foo',
  protocol: 'postgres',
  type: 'self-hosted',
  desc: '',
  hostname: '',
  addr: '',
  labelsList: [],
  ...props,
});

export const makeKube = (props: Partial<tsh.Kube>): tsh.Kube => ({
  name: 'foo',
  labelsList: [],
  uri: '/clusters/bar/kubes/foo',
  ...props,
});

export const makeLabelsList = (labels: Record<string, string>): tsh.Label[] =>
  Object.entries(labels).map(([name, value]) => ({ name, value }));

export const makeResourceResult = (
  props: Partial<ResourceSearchResult> & {
    kind: ResourceSearchResult['kind'];
    resource: ResourceSearchResult['resource'];
  }
): ResourceSearchResult => ({
  score: 0,
  labelMatches: [],
  resourceMatches: [],
  ...props,
});
