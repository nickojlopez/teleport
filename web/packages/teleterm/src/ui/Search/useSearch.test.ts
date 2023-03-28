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

import { sortResults } from './useSearch';
import { makeResult, makeServer, makeKube } from './searchResultTestHelpers';

describe('sortResults', () => {
  it('uses the displayed resource name as the tie breaker if the scores are equal', () => {
    const server = makeResult({
      kind: 'server',
      resource: makeServer({ hostname: 'z' }),
    });
    const kube = makeResult({
      kind: 'kube',
      resource: makeKube({ name: 'a' }),
    });
    const sortedResults = sortResults([server, kube], '');

    expect(sortedResults[0]).toEqual(kube);
    expect(sortedResults[1]).toEqual(server);
  });
});
