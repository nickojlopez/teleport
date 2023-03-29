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

import React, {
  useContext,
  useState,
  FC,
  useCallback,
  createContext,
  useRef,
  MutableRefObject,
} from 'react';

import { ClusterUri } from 'teleterm/ui/uri';

import { actionPicker, SearchPicker } from './pickers/pickers';

export interface SearchContext {
  inputRef: MutableRefObject<HTMLInputElement>;
  inputValue: string;
  opened: boolean;
  searchFilters: SearchFilter[];
  activePicker: SearchPicker;

  onInputValueChange(value: string): void;

  changeActivePicker(picker: SearchPicker): void;

  close(): void;

  closeAndResetInput(): void;

  open(): void;

  resetInput(): void;

  setSearchFilter(filter: SearchFilter): void;

  removeSearchFilter(filter: SearchFilter): void;
}

export interface ResourceTypeSearchFilter {
  filter: 'resource-type';
  resourceType: 'kubes' | 'servers' | 'databases';
}

export interface ClusterSearchFilter {
  filter: 'cluster';
  clusterUri: ClusterUri;
}

export type SearchFilter = ResourceTypeSearchFilter | ClusterSearchFilter;

const SearchContext = createContext<SearchContext>(null);

export const SearchContextProvider: FC = props => {
  const inputRef = useRef<HTMLInputElement>();
  const [opened, setOpened] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [activePicker, setActivePicker] = useState(actionPicker);
  const [searchFilters, setSearchFilters] = useState<SearchFilter[]>([]);

  function changeActivePicker(picker: SearchPicker): void {
    setActivePicker(picker);
    setInputValue('');
  }

  const close = useCallback(() => {
    setOpened(false);
    setActivePicker(actionPicker);
  }, []);

  const closeAndResetInput = useCallback(() => {
    close();
    setInputValue('');
  }, [close]);

  const resetInput = useCallback(() => {
    setInputValue('');
  }, []);

  function open(): void {
    setOpened(true);
    inputRef.current?.focus();
  }

  function setSearchFilter(filter: SearchFilter) {
    // UI prevents adding more than one filter of the same type
    setSearchFilters(prevState => [...prevState, filter]);
    inputRef.current?.focus();
  }

  function removeSearchFilter(filter: SearchFilter) {
    setSearchFilters(prevState => {
      const index = prevState.indexOf(filter);
      if (index >= 0) {
        const copied = [...prevState];
        copied.splice(index, 1);
        return copied;
      }
      return prevState;
    });
    inputRef.current?.focus();
  }

  return (
    <SearchContext.Provider
      value={{
        inputRef,
        inputValue,
        onInputValueChange: setInputValue,
        changeActivePicker,
        activePicker,
        searchFilters,
        setSearchFilter,
        removeSearchFilter,
        close,
        closeAndResetInput,
        resetInput,
        opened,
        open,
      }}
      children={props.children}
    />
  );
};

export const useSearchContext = () => {
  const context = useContext(SearchContext);

  if (!context) {
    throw new Error('SearchContext requires SearchContextProvider context.');
  }

  return context;
};
