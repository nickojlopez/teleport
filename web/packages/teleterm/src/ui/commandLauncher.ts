/*
Copyright 2019 Gravitational, Inc.

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

import { IAppContext } from 'teleterm/ui/types';
import { ClusterUri, KubeUri, RootClusterUri, routing } from 'teleterm/ui/uri';
import { TrackedKubeConnection } from 'teleterm/ui/services/connectionTracker';
import { Platform } from 'teleterm/mainProcess/types';

const commands = {
  // For handling "tsh ssh" executed from the command bar.
  'tsh-ssh': {
    displayName: '',
    description: '',
    run(
      ctx: IAppContext,
      args: { loginHost: string; localClusterUri: ClusterUri }
    ) {
      const { loginHost, localClusterUri } = args;
      const rootClusterUri = routing.ensureRootClusterUri(localClusterUri);
      const documentsService =
        ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);

      const doc = documentsService.createTshNodeDocumentFromLoginHost(
        localClusterUri,
        loginHost
      );
      documentsService.add(doc);
      documentsService.setLocation(doc.uri);
    },
  },

  'tsh-install': {
    displayName: '',
    description: '',
    run(ctx: IAppContext) {
      ctx.mainProcessClient.symlinkTshMacOs().then(
        isSymlinked => {
          if (isSymlinked) {
            ctx.notificationsService.notifyInfo(
              'tsh successfully installed in PATH'
            );
          }
        },
        error => {
          ctx.notificationsService.notifyError({
            title: 'Could not install tsh in PATH',
            description: `Ran into an error: ${error}`,
          });
        }
      );
    },
  },

  'tsh-uninstall': {
    displayName: '',
    description: '',
    run(ctx: IAppContext) {
      ctx.mainProcessClient.removeTshSymlinkMacOs().then(
        isRemoved => {
          if (isRemoved) {
            ctx.notificationsService.notifyInfo(
              'tsh successfully removed from PATH'
            );
          }
        },
        error => {
          ctx.notificationsService.notifyError({
            title: 'Could not remove tsh from PATH',
            description: `Ran into an error: ${error}`,
          });
        }
      );
    },
  },

  'kube-connect': {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { kubeUri: KubeUri }) {
      const documentsService =
        ctx.workspacesService.getActiveWorkspaceDocumentService();
      const kubeDoc = documentsService.createTshKubeDocument({
        kubeUri: args.kubeUri,
      });
      const connection = ctx.connectionTracker.findConnectionByDocument(
        kubeDoc
      ) as TrackedKubeConnection;
      documentsService.add({
        ...kubeDoc,
        kubeConfigRelativePath:
          connection?.kubeConfigRelativePath || kubeDoc.kubeConfigRelativePath,
      });
      documentsService.open(kubeDoc.uri);
    },
  },

  'cluster-connect': {
    displayName: '',
    description: '',
    run(
      ctx: IAppContext,
      args: { clusterUri?: RootClusterUri; onSuccess?(): void }
    ) {
      const defaultHandler = (clusterUri: RootClusterUri) => {
        ctx.commandLauncher.executeCommand('cluster-open', { clusterUri });
      };

      ctx.modalsService.openClusterConnectDialog({
        clusterUri: args.clusterUri,
        onSuccess: args.onSuccess || defaultHandler,
      });
    },
  },

  'cluster-logout': {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { clusterUri: RootClusterUri }) {
      const cluster = ctx.clustersService.findCluster(args.clusterUri);
      ctx.modalsService.openRegularDialog({
        kind: 'cluster-logout',
        clusterUri: cluster.uri,
        clusterTitle: cluster.name,
      });
    },
  },

  'cluster-open': {
    displayName: '',
    description: '',
    async run(ctx: IAppContext, args: { clusterUri: ClusterUri }) {
      const { clusterUri } = args;
      const rootCluster =
        ctx.clustersService.findRootClusterByResource(clusterUri);
      await ctx.workspacesService.setActiveWorkspace(rootCluster.uri);
      const documentsService =
        ctx.workspacesService.getWorkspaceDocumentService(rootCluster.uri);
      const doc = documentsService.findClusterDocument(clusterUri);
      if (doc) {
        documentsService.open(doc.uri);
      } else {
        const newDoc = documentsService.createClusterDocument({ clusterUri });
        documentsService.add(newDoc);
        documentsService.open(newDoc.uri);
      }
    },
  },
};

const autocompleteCommands: {
  displayName: string;
  description: string;
  platforms?: Array<Platform>;
}[] = [
  {
    displayName: 'tsh ssh',
    description: 'Run shell or execute a command on a remote SSH node',
  },
  {
    displayName: 'tsh proxy db',
    description: 'Start a local proxy for a database connection',
  },
  {
    displayName: 'tsh install',
    description: 'Install tsh in PATH',
    platforms: ['darwin'],
  },
  {
    displayName: 'tsh uninstall',
    description: 'Uninstall tsh from PATH',
    platforms: ['darwin'],
  },
];

export class CommandLauncher {
  appContext: IAppContext;

  constructor(appContext: IAppContext) {
    this.appContext = appContext;
  }

  executeCommand<T extends CommandName>(name: T, args: CommandArgs<T>) {
    commands[name].run(this.appContext, args as any);
    return undefined;
  }

  getAutocompleteCommands() {
    const { platform } = this.appContext.mainProcessClient.getRuntimeSettings();

    return autocompleteCommands.filter(command => {
      const platforms = command.platforms;
      return !command.platforms || platforms.includes(platform);
    });
  }
}

type CommandName = keyof typeof commands;
type CommandRegistry = typeof commands;
type CommandArgs<T extends CommandName> = Parameters<
  CommandRegistry[T]['run']
>[1];
