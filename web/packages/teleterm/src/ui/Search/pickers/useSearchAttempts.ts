import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { makeEmptyAttempt, mapAttempt, useAsync } from 'shared/hooks/useAsync';

import {
  sortResults,
  useFilterSearch,
  useResourceSearch,
} from 'teleterm/ui/Search/useSearch';
import { mapToActions } from 'teleterm/ui/Search/actions';
import Logger from 'teleterm/logger';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useSearchContext } from 'teleterm/ui/Search/SearchContext';

export function useSearchAttempts() {
  const searchLogger = useRef(new Logger('search'));
  const ctx = useAppContext();
  const searchContext = useSearchContext();
  const { inputValue, searchFilters } = searchContext;
  const [prevInputValue, setPrevInputValue] = useState<string>(inputValue);
  const debouncedInputValue = useDebounce(inputValue, 200);

  const [resourceSearchAttempt, runResourceSearch, setResourceSearchAttempt] =
    useAsync(useResourceSearch());
  const [filterSearchAttempt, runFilterSearch, setFilterSearchAttempt] =
    useAsync(useFilterSearch());

  ctx.workspacesService.useState();

  const resetAttempts = useCallback(() => {
    setResourceSearchAttempt(makeEmptyAttempt());
    setFilterSearchAttempt(makeEmptyAttempt());
  }, [setResourceSearchAttempt, setFilterSearchAttempt]);

  const resourceActionsAttempt = useMemo(
    () =>
      mapAttempt(resourceSearchAttempt, ({ results, search }) => {
        const sortedResults = sortResults(results, search);
        searchLogger.current.info('results for', search, sortedResults);

        return mapToActions(ctx, searchContext, sortedResults);
      }),
    [ctx, resourceSearchAttempt, searchContext]
  );

  const filterActionsAttempt = useMemo(
    () =>
      mapAttempt(filterSearchAttempt, ({ results }) =>
        // TODO(gzdunek): filters are sorted inline, should be done here to align with resource search
        mapToActions(ctx, searchContext, results)
      ),
    [ctx, filterSearchAttempt, searchContext]
  );

  if (inputValue !== prevInputValue) {
    // Reset both attempts as soon as the input changes. If we didn't do that, then the resource
    // search attempt would only get updated on debounce. This could lead to the following scenario:
    //
    // 1. You type in `foo`, wait for the results to show up.
    // 2. You clear the input and quickly type in `bar`.
    // 3. Now you see the stale results for `foo`, because the debounce didn't kick in yet.
    setPrevInputValue(inputValue);
    resetAttempts();
  }

  useEffect(() => {
    if (debouncedInputValue) {
      runResourceSearch(debouncedInputValue, searchFilters);
    }
  }, [debouncedInputValue, runResourceSearch, searchFilters]);

  // TODO(ravicious): Run this as soon as the input value changes, not within a useEffect.
  // If you consider moving runFilterSearch within the same conditional as setPrevInputValue and
  // resetAttempts, then you might need to change the initial prevInputValue to be null instead of
  // ''. Otherwise runFilterSearch won't run on the initial render.
  //
  // But perhaps there's a way to have it only on input value change and then run it once on render?
  // https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes
  useEffect(() => {
    runFilterSearch(inputValue, searchFilters);
  }, [searchFilters, inputValue, runFilterSearch]);

  return {
    resetAttempts,
    attempts: [filterActionsAttempt, resourceActionsAttempt],
  };
}

function useDebounce<T>(value: T, delay: number): T {
  // State and setters for debounced value
  const [debouncedValue, setDebouncedValue] = useState(value);
  useEffect(
    () => {
      // Update debounced value after delay
      const handler = setTimeout(() => setDebouncedValue(value), delay);
      // Cancel the timeout if value changes (also on delay change or unmount)
      // This is how we prevent debounced value from updating if value is changed ...
      // .. within the delay period. Timeout gets cleared and restarted.
      return () => clearTimeout(handler);
    },
    [value, delay] // Only re-call effect if value or delay changes
  );
  return debouncedValue;
}
