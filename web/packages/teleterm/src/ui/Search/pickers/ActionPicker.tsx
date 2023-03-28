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

import React, { useCallback } from 'react';
import styled from 'styled-components';
import { Box, Flex, Label as DesignLabel, Text } from 'design';
import * as icons from 'design/Icon';
import { Highlight } from 'shared/components/Highlight';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  ResourceMatch,
  SearchResult,
  ResourceSearchResult,
  SearchResultDatabase,
  SearchResultKube,
  SearchResultServer,
  SearchResultCluster,
  SearchResultResourceType,
} from 'teleterm/ui/Search/searchResult';
import * as tsh from 'teleterm/services/tshd/types';
import * as uri from 'teleterm/ui/uri';

import { SearchAction } from '../actions';
import { useSearchContext } from '../SearchContext';

import { useSearchAttempts } from './useSearchAttempts';

import { getParameterPicker } from './pickers';
import { ResultList, EmptyListCopy } from './ResultList';

export function ActionPicker() {
  const ctx = useAppContext();
  const { clustersService } = ctx;
  ctx.clustersService.useState();

  const {
    inputValue,
    changeActivePicker,
    close,
    resetInput,
    closeAndResetInput,
  } = useSearchContext();
  const { attempts, resetAttempts } = useSearchAttempts();

  const getClusterName = useCallback(
    (resourceUri: uri.ResourceUri) => {
      const clusterUri = uri.routing.ensureClusterUri(resourceUri);
      const cluster = clustersService.findCluster(clusterUri);

      return cluster ? cluster.name : uri.routing.parseClusterName(resourceUri);
    },
    [clustersService]
  );

  const onPick = useCallback(
    (action: SearchAction) => {
      resetAttempts();

      if (action.type === 'simple-action') {
        action.perform();
        if (action.preventAutoClose === true) {
          resetInput();
        } else {
          closeAndResetInput();
        }
      }
      if (action.type === 'parametrized-action') {
        changeActivePicker(getParameterPicker(action));
      }
    },
    [changeActivePicker, resetAttempts, closeAndResetInput, resetInput]
  );

  if (!inputValue) {
    return (
      <EmptyListCopy>
        <Text>
          <ul>
            <li>Separate the search terms with space.</li>
            <li>
              Resources that match the query the most will appear at the top.
            </li>
            <li>
              Selecting a search result will connect to the resource in a new
              tab.
            </li>
          </ul>
        </Text>
      </EmptyListCopy>
    );
  }

  return (
    <ResultList<SearchAction>
      attempts={attempts}
      onPick={onPick}
      onBack={close}
      render={item => {
        const Component = ComponentMap[item.searchResult.kind];
        return {
          key:
            item.searchResult.kind !== 'resource-type-filter'
              ? item.searchResult.resource.uri
              : item.searchResult.resource,
          Component: (
            <Component
              searchResult={item.searchResult}
              getClusterName={getClusterName}
            />
          ),
        };
      }}
      NoResultsComponent={
        <EmptyListCopy>
          <Text>No matching results found.</Text>
        </EmptyListCopy>
      }
    />
  );
}

export const ComponentMap: Record<
  SearchResult['kind'],
  React.FC<SearchResultItem<SearchResult>>
> = {
  server: ServerItem,
  kube: KubeItem,
  database: DatabaseItem,
  'cluster-filter': NarrowDownByCluster,
  'resource-type-filter': NarrowDownByResourceType,
};

type SearchResultItem<T> = {
  searchResult: T;
  getClusterName: (uri: uri.ResourceUri) => string;
};

function NarrowDownByCluster(props: SearchResultItem<SearchResultCluster>) {
  return (
    <Flex alignItems="flex-start" minWidth="300px">
      <SquareIconBackground color="#4DB2F0">
        <icons.CardView fontSize="20px" />
      </SquareIconBackground>
      <Flex flexDirection="column" ml={1} flex={1}>
        <Box mr={2}>
          {'Search only in '}
          <Highlight
            text={props.searchResult.resource.name}
            keywords={[props.searchResult.nameMatch]}
          />
        </Box>
      </Flex>
    </Flex>
  );
}

function NarrowDownByResourceType(
  props: SearchResultItem<SearchResultResourceType>
) {
  return (
    <Flex alignItems="flex-start" minWidth="300px">
      <SquareIconBackground color="#512FC9">
        <icons.Cluster fontSize="20px" />
      </SquareIconBackground>
      <Flex flexDirection="column" ml={1} flex={1}>
        <Box mr={2}>
          {'Search only for '}
          <Highlight
            text={props.searchResult.resource}
            keywords={[props.searchResult.nameMatch]}
          />
        </Box>
      </Flex>
    </Flex>
  );
}

export function ServerItem(props: SearchResultItem<SearchResultServer>) {
  const { searchResult } = props;
  const server = searchResult.resource;
  const hasUuidMatches = searchResult.resourceMatches.some(
    match => match.field === 'name'
  );

  return (
    <Flex flexDirection="column" minWidth="300px" gap={1}>
      <Flex justifyContent="space-between" alignItems="center">
        <Flex alignItems="center" gap={1} flex="1 0">
          <SquareIconBackground color="#c05b9e">
            <icons.Server />
          </SquareIconBackground>
          <Text typography="body1">
            Connect over SSH to{' '}
            <strong>
              <HighlightField field="hostname" searchResult={searchResult} />
            </strong>
          </Text>
        </Flex>
        <Box>
          <Text typography="body2" fontSize={0}>
            {props.getClusterName(server.uri)}
          </Text>
        </Box>
      </Flex>

      <Labels searchResult={searchResult}>
        <ResourceFields>
          {server.tunnel ? (
            <span title="This node is connected to the cluster through a reverse tunnel">
              ↵ tunnel
            </span>
          ) : (
            <span>
              <HighlightField field="addr" searchResult={searchResult} />
            </span>
          )}

          {hasUuidMatches && (
            <span>
              UUID:{' '}
              <HighlightField field={'name'} searchResult={searchResult} />
            </span>
          )}
        </ResourceFields>
      </Labels>
    </Flex>
  );
}

export function DatabaseItem(props: SearchResultItem<SearchResultDatabase>) {
  const { searchResult } = props;
  const db = searchResult.resource;

  const $resourceFields = (
    <ResourceFields>
      <span
        css={`
          flex-shrink: 0;
        `}
      >
        <HighlightField field="type" searchResult={searchResult} />
        /
        <HighlightField field="protocol" searchResult={searchResult} />
      </span>
      {db.desc && (
        <span
          css={`
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          `}
        >
          <HighlightField field="desc" searchResult={searchResult} />
        </span>
      )}
    </ResourceFields>
  );

  return (
    <Flex flexDirection="column" minWidth="300px" gap={1}>
      <Flex justifyContent="space-between" alignItems="center">
        <Flex alignItems="center" gap={1} flex="1 0">
          <SquareIconBackground
            color="#4ab9c9"
            // The database icon is different than ssh and kube icons for some reason.
            css={`
              padding-left: 5px;
              padding-top: 5px;
            `}
          >
            <icons.Database />
          </SquareIconBackground>
          <Text typography="body1">
            Set up a db connection for{' '}
            <strong>
              <HighlightField field="name" searchResult={searchResult} />
            </strong>
          </Text>
        </Flex>
        <Box>
          <Text typography="body2" fontSize={0}>
            {props.getClusterName(db.uri)}
          </Text>
        </Box>
      </Flex>

      {/* If the description is long, put the resource fields on a separate line.
          Otherwise show the resource fields and the labels together in a single line.
       */}
      {db.desc.length >= 30 ? (
        <>
          {$resourceFields}
          <Labels searchResult={searchResult} />
        </>
      ) : (
        <Labels searchResult={searchResult}>{$resourceFields}</Labels>
      )}
    </Flex>
  );
}

export function KubeItem(props: SearchResultItem<SearchResultKube>) {
  const { searchResult } = props;

  return (
    <Flex flexDirection="column" minWidth="300px" gap={1}>
      <Flex justifyContent="space-between" alignItems="center">
        <Flex alignItems="center" gap={1} flex="1 0">
          <SquareIconBackground color="#326ce5">
            <icons.Kubernetes />
          </SquareIconBackground>
          <Text typography="body1">
            Log in to Kubernetes cluster{' '}
            <strong>
              <HighlightField field="name" searchResult={searchResult} />
            </strong>
          </Text>
        </Flex>
        <Box>
          <Text typography="body2" fontSize={0}>
            {props.getClusterName(searchResult.resource.uri)}
          </Text>
        </Box>
      </Flex>

      <Labels searchResult={searchResult} />
    </Flex>
  );
}

function Labels(
  props: React.PropsWithChildren<{
    searchResult: ResourceSearchResult;
  }>
) {
  const { searchResult } = props;

  // Label name to score.
  const scoreMap: Map<string, number> = new Map();
  searchResult.labelMatches.forEach(match => {
    const currentScore = scoreMap.get(match.labelName) || 0;
    scoreMap.set(match.labelName, currentScore + match.score);
  });

  const sortedLabelsList = [...searchResult.resource.labelsList];
  sortedLabelsList.sort(
    (a, b) =>
      // Highest score first.
      (scoreMap.get(b.name) || 0) - (scoreMap.get(a.name) || 0)
  );

  return (
    <LabelsFlex>
      {props.children}
      {sortedLabelsList.map(label => (
        <Label
          key={label.name + label.value}
          searchResult={searchResult}
          label={label}
        />
      ))}
    </LabelsFlex>
  );
}

const LabelsFlex = styled(Flex).attrs({ gap: 1 })`
  overflow-x: hidden;
  flex-wrap: nowrap;
  align-items: baseline;

  // Make the children not shrink, otherwise they would shrink in attempt to render all labels in
  // the same row.
  & > * {
    flex-shrink: 0;
  }
`;

const ResourceFields = styled(Flex).attrs({ gap: 1 })`
  color: ${props => props.theme.colors.text.primary};
  font-size: ${props => props.theme.fontSizes[0]}px;
`;

function Label(props: {
  searchResult: ResourceSearchResult;
  label: tsh.Label;
}) {
  const { searchResult: item, label } = props;
  const labelMatches = item.labelMatches.filter(
    match => match.labelName == label.name
  );
  const nameMatches = labelMatches
    .filter(match => match.kind === 'label-name')
    .map(match => match.searchTerm);
  const valueMatches = labelMatches
    .filter(match => match.kind === 'label-value')
    .map(match => match.searchTerm);

  return (
    <DesignLabel
      key={label.name}
      kind="secondary"
      title={`${label.name}: ${label.value}`}
    >
      <Highlight text={label.name} keywords={nameMatches} />:{' '}
      <Highlight text={label.value} keywords={valueMatches} />
    </DesignLabel>
  );
}

function HighlightField(props: {
  searchResult: ResourceSearchResult;
  field: ResourceMatch<ResourceSearchResult['kind']>['field'];
}) {
  // `as` used as a workaround for a TypeScript issue.
  // https://github.com/microsoft/TypeScript/issues/33591
  const keywords = (
    props.searchResult.resourceMatches as ResourceMatch<
      ResourceSearchResult['kind']
    >[]
  )
    .filter(match => match.field === props.field)
    .map(match => match.searchTerm);

  return (
    <Highlight
      text={props.searchResult.resource[props.field]}
      keywords={keywords}
    />
  );
}

const SquareIconBackground = styled(Box)`
  background: ${props => props.color};
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 24px;
  width: 24px;
  border-radius: 2px;
  padding: 4px;
  font-size: 18px;
`;
