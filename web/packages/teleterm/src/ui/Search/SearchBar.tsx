/**
 * Copyright 2023 Gravitational, Inc.
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

import React, { useRef, useEffect } from 'react';
import styled from 'styled-components';
import { Box, ButtonPrimary, Flex } from 'design';
import { space, width, color, height } from 'styled-system';

import {
  SearchContextProvider,
  useSearchContext,
} from 'teleterm/ui/Search/SearchContext';
import { KeyboardShortcutAction } from 'teleterm/services/config';
import {
  useKeyboardShortcutFormatters,
  useKeyboardShortcuts,
} from 'teleterm/ui/services/keyboardShortcuts';
import { routing } from 'teleterm/ui/uri';

import { useAppContext } from '../appContextProvider';

import { actionPicker } from './pickers/pickers';

const OPEN_COMMAND_BAR_SHORTCUT_ACTION: KeyboardShortcutAction =
  'openCommandBar';

export function SearchBarConnected() {
  return (
    <SearchContextProvider>
      <SearchBar />
    </SearchContextProvider>
  );
}

export function SearchBar() {
  const listRef = useRef<HTMLElement>();
  const containerRef = useRef<HTMLElement>();
  const { getAccelerator } = useKeyboardShortcutFormatters();
  const {
    activePicker,
    inputValue,
    onInputValueChange,
    inputRef,
    opened,
    open,
    close,
    searchFilters,
    removeSearchFilter,
  } = useSearchContext();
  const ctx = useAppContext();
  ctx.clustersService.useState();

  useKeyboardShortcuts({
    [OPEN_COMMAND_BAR_SHORTCUT_ACTION]: () => {
      open();
    },
  });

  // TODO: bring back onBlur
  useEffect(() => {
    const onClickOutside = e => {
      if (!e.composedPath().includes(containerRef.current)) {
        close();
      }
    };
    window.addEventListener('click', onClickOutside);
    return () => window.removeEventListener('click', onClickOutside);
  }, [close]);

  function handleOnFocus() {
    if (!opened) {
      open();
    }
  }

  // TODO(gzdunek): this will be probably moved to `ActionPicker` (altogether with `Input`)
  const filterButtons = searchFilters.map(s => {
    if (s.filter === 'resource-type') {
      return (
        <ButtonPrimary
          m="4px"
          mr="2px"
          px="8px"
          size="small"
          key="resource-type"
          onClick={() => removeSearchFilter(s)}
        >
          {s.resourceType}
        </ButtonPrimary>
      );
    }
    if (s.filter === 'cluster') {
      const clusterName =
        ctx.clustersService.findCluster(s.clusterUri)?.name ||
        routing.parseClusterName(s.clusterUri);
      return (
        <ButtonPrimary
          m="4px"
          mr="2px"
          px="8px"
          size="small"
          title={clusterName}
          css={`
            max-width: 130px;
            text-overflow: ellipsis;
            white-space: nowrap;
            overflow: hidden;
            display: block;
          `}
          key="cluster"
          onClick={() => removeSearchFilter(s)}
        >
          {clusterName}
        </ButtonPrimary>
      );
    }
  });

  function handleKeyDown(e: React.KeyboardEvent) {
    const { length } = searchFilters;
    if (e.key === 'Backspace' && inputValue === '' && length) {
      removeSearchFilter(searchFilters[length - 1]);
    }
  }

  return (
    <Flex
      css={`
        position: relative;
        flex: 4;
        flex-shrink: 1;
        min-width: calc(${props => props.theme.space[7]}px * 2);
        height: 100%;
        background: ${props => props.theme.colors.primary.light};
        border: 1px ${props => props.theme.colors.action.disabledBackground}
          solid;
      `}
      justifyContent="center"
      ref={containerRef}
      onFocus={handleOnFocus}
    >
      {opened && activePicker === actionPicker && <Flex>{filterButtons}</Flex>}
      <Input
        ref={inputRef}
        placeholder={activePicker.placeholder}
        value={inputValue}
        onKeyDown={handleKeyDown}
        onChange={e => {
          onInputValueChange(e.target.value);
        }}
        spellCheck={false}
      />
      {!opened && (
        <Shortcut>{getAccelerator(OPEN_COMMAND_BAR_SHORTCUT_ACTION)}</Shortcut>
      )}
      {opened && (
        <StyledGlobalSearchResults ref={listRef}>
          {activePicker.picker}
        </StyledGlobalSearchResults>
      )}
    </Flex>
  );
}

const Input = styled.input(props => {
  const { theme } = props;
  return {
    height: '100%',
    background: theme.colors.primary.dark,
    boxSizing: 'border-box',
    color: theme.colors.text.primary,
    width: '100%',
    outline: 'none',
    border: 'none',
    padding: `${theme.space[1]}px ${theme.space[2]}px`,
    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText,
      background: theme.colors.primary.light,

      opacity: 1,
    },
    '::placeholder': {
      color: theme.colors.text.secondary,
    },

    ...space(props),
    ...width(props),
    ...height(props),
    ...color(props),
  };
});

// TODO: Make the Shortcut cover the placeholder. See how QuickInput Shortcut is implemented where
// it covers the placeholder.
// TODO: Center the Shortcut.
const Shortcut = styled(Box)`
  position: absolute;
  right: 12px;
  top: 10px;
  padding: 2px 3px;
  color: ${({ theme }) => theme.colors.text.secondary};
  background-color: ${({ theme }) => theme.colors.primary.light};
  line-height: 12px;
  font-size: 12px;
  border-radius: 2px;
`;

const StyledGlobalSearchResults = styled.div(({ theme }) => {
  return {
    boxShadow: '8px 8px 18px rgb(0 0 0)',
    color: theme.colors.primary.contrastText,
    background: theme.colors.primary.light,
    boxSizing: 'border-box',
    // Account for border.
    width: 'calc(100% + 2px)',
    // Careful, this is hardcoded based on the input height.
    marginTop: '38px',
    display: 'block',
    position: 'absolute',
    border: '1px solid ' + theme.colors.action.hover,
    fontSize: '12px',
    listStyle: 'none outside none',
    textShadow: 'none',
    zIndex: '1000',
    maxHeight: '350px',
    overflow: 'auto',
    // Hardcoded to height of the shortest item.
    minHeight: '42px',
  };
});
