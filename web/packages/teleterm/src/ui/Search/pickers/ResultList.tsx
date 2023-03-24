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

import React, {
  ReactElement,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import styled from 'styled-components';

import { Attempt } from 'shared/hooks/useAsync';
import { Box } from 'design';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

type ResultListProps<T> = {
  attempt: Attempt<T[]>;
  /**
   * extraItems is an array of extra results that get render irrelevant of the attempt status.
   */
  extraItems?: T[];
  /**
   * NoResultsComponent is the element that's going to be rendered instead of the list if the
   * attempt has successfully finished but there's no results to show.
   */
  NoResultsComponent?: ReactElement;
  onPick(item: T): void;
  onBack(): void;
  render(item: T): { Component: ReactElement; key: string };
};

export function ResultList<T>(props: ResultListProps<T>) {
  const {
    attempt,
    extraItems = [],
    NoResultsComponent,
    onPick,
    onBack,
  } = props;
  const activeItemRef = useRef<HTMLDivElement>();
  const [activeItemIndex, setActiveItemIndex] = useState(0);
  const shouldShowNoResultsCopy =
    NoResultsComponent &&
    attempt.status === 'success' &&
    attempt.data.length === 0;

  const items = useMemo(
    () =>
      attempt.status === 'success'
        ? [...extraItems, ...attempt.data]
        : extraItems,
    [attempt.status, attempt.data, extraItems]
  );

  // Reset the active item index if it's greater than the number of available items.
  // This can happen in cases where the user selects the nth item and then filters the list so that
  // there's only one item.
  if (activeItemIndex !== 0 && activeItemIndex >= items.length) {
    setActiveItemIndex(0);
  }

  useEffect(() => {
    const handleArrowKey = (e: KeyboardEvent, nudge: number) => {
      const next = getNext(activeItemIndex + nudge, items.length);
      setActiveItemIndex(next);
      // `false` - bottom of the element will be aligned to the bottom of the visible area of the scrollable ancestor
      activeItemRef.current?.scrollIntoView(false);
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'Enter': {
          e.stopPropagation();
          e.preventDefault();

          const item = items[activeItemIndex];
          if (item) {
            onPick(item);
          }
          break;
        }
        case 'Escape': {
          onBack();
          break;
        }
        case 'ArrowUp':
          e.stopPropagation();
          e.preventDefault();

          handleArrowKey(e, -1);
          break;
        case 'ArrowDown':
          e.stopPropagation();
          e.preventDefault();

          handleArrowKey(e, 1);
          break;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [items, attempt.status, onPick, onBack, activeItemIndex]);

  return (
    <>
      {attempt.status === 'processing' && (
        <div
          style={{
            position: 'absolute',
            top: 0,
            height: '1px',
            left: 0,
            right: 0,
          }}
        >
          <LinearProgress transparentBackground={true} />
        </div>
      )}
      {items.map((r, index) => {
        const isActive = index === activeItemIndex;
        const { Component, key } = props.render(r);

        return (
          <StyledItem
            ref={isActive ? activeItemRef : null}
            $active={isActive}
            key={key}
            onClick={() => props.onPick(r)}
          >
            {Component}
          </StyledItem>
        );
      })}
      {shouldShowNoResultsCopy && NoResultsComponent}
    </>
  );
}

const StyledItem = styled.div(({ theme, $active }) => {
  return {
    '&:hover, &:focus': {
      cursor: 'pointer',
      background: theme.colors.primary.lighter,
    },
    '& mark': {
      color: 'inherit',
      backgroundColor: theme.colors.secondary.light,
    },

    borderBottom: `2px solid ${theme.colors.primary.main}`,
    padding: `${theme.space[2]}px`,
    color: theme.colors.primary.contrastText,
    background: $active
      ? theme.colors.primary.lighter
      : theme.colors.primary.light,
  };
});

export const EmptyListCopy = styled(Box)`
  width: 100%;
  height: 100%;
  padding: ${props => props.theme.space[2]}px;
  line-height: 1.5em;
  ul {
    margin: 0;
    padding-inline-start: 2em;
  }
`;

function getNext(selectedIndex = 0, max = 0) {
  let index = selectedIndex % max;
  if (index < 0) {
    index += max;
  }
  return index;
}
