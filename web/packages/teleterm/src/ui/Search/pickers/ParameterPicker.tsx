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

import React, { useCallback, useEffect } from 'react';
import { Highlight } from 'shared/components/Highlight';
import { mapAttempt, useAsync } from 'shared/hooks/useAsync';

import { useSearchContext } from '../SearchContext';
import { ParametrizedAction } from '../actions';

import { ResultList } from './ResultList';
import { actionPicker } from './pickers';

interface ParameterPickerProps {
  action: ParametrizedAction;
}

export function ParameterPicker(props: ParameterPickerProps) {
  const { inputValue, close, changeActivePicker } = useSearchContext();
  const [suggestionsAttempt, fetch] = useAsync(
    props.action.parameter.getSuggestions
  );

  useEffect(() => {
    fetch();
  }, [props.action]);

  const attempt = mapAttempt(suggestionsAttempt, suggestions =>
    suggestions.filter(
      v =>
        v.toLocaleLowerCase().includes(inputValue.toLocaleLowerCase()) &&
        v !== inputValue
    )
  );

  let extraItems: string[] = [];
  if (inputValue) {
    extraItems = [inputValue];
  }

  const onPick = useCallback(
    (item: string) => {
      props.action.perform(item);
      close();
    },
    [close, props.action]
  );

  const onBack = useCallback(() => {
    changeActivePicker(actionPicker);
  }, [changeActivePicker]);

  return (
    <ResultList<string>
      attempt={attempt}
      extraItems={extraItems}
      onPick={onPick}
      onBack={onBack}
      render={item => ({
        key: item,
        Component: <Highlight text={item} keywords={[inputValue]}></Highlight>,
      })}
    />
  );
}
