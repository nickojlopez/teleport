import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
} from 'react';
import { makeEmptyAttempt, mapAttempt, useAsync } from 'shared/hooks/useAsync';

import { debounce } from 'shared/utils/highbar';

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

  const [resourceSearchAttempt, runResourceSearch, setResourceSearchAttempt] =
    useAsync(useResourceSearch());
  const [filterSearchAttempt, runFilterSearch, setFilterSearchAttempt] =
    useAsync(useFilterSearch());

  const runResourceSearchDebounced = useDebounce(runResourceSearch, 200);

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
  if (inputValue === '' && resourceActionsAttempt.status !== '') {
    setResourceSearchAttempt(makeEmptyAttempt());
  }

  useEffect(() => {
    runFilterSearch(inputValue, searchFilters);

    if (inputValue) {
      runResourceSearchDebounced(inputValue, searchFilters);
    }
  }, [searchFilters, inputValue, runFilterSearch, runResourceSearchDebounced]);

  return {
    resetAttempts,
    attempts: [filterActionsAttempt, resourceActionsAttempt],
  };
}

function useDebounce<Args extends unknown[], ReturnValue>(
  callback: (...args: Args) => ReturnValue,
  delay: number
) {
  const callbackRef = useRef(callback);
  useLayoutEffect(() => {
    callbackRef.current = callback;
  });
  return useMemo(
    () => debounce((...args: Args) => callbackRef.current(...args), delay),
    [delay]
  );
}
