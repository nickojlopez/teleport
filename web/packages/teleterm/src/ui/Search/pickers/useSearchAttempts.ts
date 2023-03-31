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

  // Both states are used by mapToActions.
  ctx.workspacesService.useState();
  ctx.clustersService.useState();

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

  useEffect(() => {
    // Reset both attempts as soon as the input changes. If we didn't do that, then the resource
    // search attempt would only get updated on debounce. This could lead to the following scenario:
    //
    // 1. You type in `foo`, wait for the results to show up.
    // 2. You clear the input and quickly type in `bar`.
    // 3. Now you see the stale results for `foo`, because the debounce didn't kick in yet.
    resetAttempts();

    runFilterSearch(inputValue, searchFilters);
    runResourceSearchDebounced(inputValue, searchFilters);
  }, [
    inputValue,
    searchFilters,
    resetAttempts,
    runFilterSearch,
    runResourceSearchDebounced,
  ]);

  return [filterActionsAttempt, resourceActionsAttempt];
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
