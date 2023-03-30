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

import React from 'react';
import styled from 'styled-components';
import { Flex } from 'design';

import QuickInput from '../QuickInput';
import { SearchBar } from '../Search';
import { useAppContext } from '../appContextProvider';

import { Connections } from './Connections';
import { Clusters } from './Clusters';
import { Identity } from './Identity';
import { MoreOptions } from './MoreOptions';

export function TopBar() {
  const { configService } = useAppContext();
  const isSearchBarEnabled = configService.get('feature.searchBar').value;

  return (
    <Grid>
      <JustifyLeft>
        <Connections />
      </JustifyLeft>
      <CentralContainer>
        <Clusters />
        {isSearchBarEnabled ? <SearchBar /> : <QuickInput />}
      </CentralContainer>
      <JustifyRight>
        <MoreOptions />
        <Identity />
      </JustifyRight>
    </Grid>
  );
}

const Grid = styled(Flex).attrs({ gap: 3, py: 2, px: 3 })`
  background: ${props => props.theme.colors.primary.main};
  width: 100%;
  height: 56px;
  align-items: center;
  justify-content: space-between;
`;

const CentralContainer = styled(Flex).attrs({ gap: 3 })`
  flex: 1;
  align-items: center;
  justify-content: center;
  height: 100%;
  max-width: calc(${props => props.theme.space[10]}px * 9);
`;

const JustifyLeft = styled.div`
  display: flex;
  justify-self: start;
  align-items: center;
  height: 100%;
`;

const JustifyRight = styled.div`
  display: flex;
  justify-self: end;
  align-items: center;
  height: 100%;
`;
