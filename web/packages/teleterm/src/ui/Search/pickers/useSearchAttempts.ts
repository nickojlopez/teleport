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
  const debouncedInputValue = useDebounce(inputValue, 200);

  const [resourceSearchAttempt, runResourceSearch, setResourceSearchAttempt] =
    useAsync(useResourceSearch());
  const [filterSearchAttempt, runFilterSearch, setFilterSearchAttempt] =
    useAsync(useFilterSearch());

  ctx.workspacesService.useState();

  const resetAttempts = useCallback(() => {
    setResourceSearchAttempt(makeEmptyAttempt());
    setFilterSearchAttempt(makeEmptyAttempt());
  }, []);

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

  // Reset the attempt if input gets cleaned. If we did that in useEffect on debouncedInputValue,
  // then if you typed in something, then cleared the input and started typing something new,
  // you'd see stale results for a brief second.
  if (
    inputValue === '' &&
    resourceActionsAttempt.status !== '' &&
    filterActionsAttempt.status !== ''
  ) {
    resetAttempts();
  }

  useEffect(() => {
    if (debouncedInputValue) {
      runResourceSearch(debouncedInputValue, searchFilters);
    }
  }, [debouncedInputValue, runResourceSearch, searchFilters]);

  useEffect(() => {
    if (inputValue) {
      runFilterSearch(inputValue, searchFilters);
    }
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
