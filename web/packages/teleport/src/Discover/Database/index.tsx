/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { ResourceKind, Finished } from 'teleport/Discover/Shared';
import { ResourceViewConfig } from 'teleport/Discover/flow';
import { DatabaseWrapper } from 'teleport/Discover/Database/DatabaseWrapper';
import {
  ResourceSpec,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';

import { CreateDatabase } from 'teleport/Discover/Database/CreateDatabase';
import { SetupAccess } from 'teleport/Discover/Database/SetupAccess';
import { DownloadScript } from 'teleport/Discover/Database/DownloadScript';
import { MutualTls } from 'teleport/Discover/Database/MutualTls';
import { TestConnection } from 'teleport/Discover/Database/TestConnection';
import { IamPolicy } from 'teleport/Discover/Database/IamPolicy';
import { DiscoverEvent } from 'teleport/services/userEvent';

export const DatabaseResource: ResourceViewConfig<ResourceSpec> = {
  kind: ResourceKind.Database,
  wrapper(component: React.ReactNode) {
    return <DatabaseWrapper>{component}</DatabaseWrapper>;
  },
  views(resource) {
    let configureResourceViews;
    if (resource && resource.dbMeta) {
      switch (resource.dbMeta.location) {
        case DatabaseLocation.Aws:
          configureResourceViews = [
            {
              title: 'Register a Database',
              component: CreateDatabase,
              eventName: DiscoverEvent.DatabaseRegister,
            },
            {
              title: 'Deploy Database Service',
              component: DownloadScript,
              eventName: DiscoverEvent.DeployService,
            },
            {
              title: 'Configure IAM Policy',
              component: IamPolicy,
              eventName: DiscoverEvent.DatabaseConfigureIAMPolicy,
            },
          ];

          break;

        case DatabaseLocation.SelfHosted:
          configureResourceViews = [
            {
              title: 'Register a Database',
              component: CreateDatabase,
              eventName: DiscoverEvent.DatabaseRegister,
            },
            {
              title: 'Deploy Database Service',
              component: DownloadScript,
              eventName: DiscoverEvent.DeployService,
            },
            {
              title: 'Configure mTLS',
              component: MutualTls,
              eventName: DiscoverEvent.DatabaseConfigureMTLS,
            },
          ];

          break;
      }
    }

    return [
      {
        title: 'Configure Resource',
        views: configureResourceViews,
      },
      {
        title: 'Set Up Access',
        component: SetupAccess,
        eventName: DiscoverEvent.PrincipalsConfigure,
      },
      {
        title: 'Test Connection',
        component: TestConnection,
        eventName: DiscoverEvent.TestConnection,
        manuallyEmitSuccessEvent: true,
      },
      {
        title: 'Finished',
        component: Finished,
        eventName: DiscoverEvent.Completed,
        hide: true,
      },
    ];
  },
};
