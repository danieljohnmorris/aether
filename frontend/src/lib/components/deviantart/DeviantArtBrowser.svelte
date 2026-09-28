<script lang="ts">
    import {onMount} from 'svelte';
    import DeviantArtFilters from './DeviantArtFilters.svelte';
    import WallpaperGrid from '$lib/components/wallhaven/WallpaperGrid.svelte';
    import type {wallhaven} from '../../../../wailsjs/go/models';
    import {
        getResults,
        getIsSearching,
        getIsLoadingMore,
        getHasMore,
        getSearchError,
        getQuery,
        getTotal,
        setQuery,
        search,
        loadMore,
        initializeSearch,
    } from '$lib/stores/deviantart.svelte';
    import {observeIntersection} from '$lib/utils/intersection';
    import EmptyState from '$lib/components/shared/EmptyState.svelte';
    import LoadingState from '$lib/components/shared/LoadingState.svelte';
    import SearchIcon from '$lib/components/shared/SearchIcon.svelte';

    let scrollContainer = $state<HTMLDivElement | null>(null);
    let sentinel = $state<HTMLDivElement | null>(null);

    // The shared card renders the thumbnail/resolution but not the artist, so
    // surface the authors of the current page here.
    const authors = $derived(
        Array.from(
            new Set(
                getResults()
                    .map(r => r.author)
                    .filter((a): a is string => !!a)
            )
        )
    );

    onMount(() => {
        void initializeSearch();
    });

    // Re-observe on every results change so the sentinel fires again even if it
    // stays intersecting — IntersectionObserver only reports *transitions*, so
    // a sentinel that's permanently in-view (first page smaller than viewport)
    // would otherwise trigger only once.
    $effect(() => {
        if (!sentinel || !scrollContainer) return;
        getResults().length;
        return observeIntersection(
            sentinel,
            entry => {
                if (
                    entry.isIntersecting &&
                    getHasMore() &&
                    !getIsSearching() &&
                    !getIsLoadingMore() &&
                    !getSearchError()
                ) {
                    loadMore();
                }
            },
            {root: scrollContainer, rootMargin: '400px 0px'}
        );
    });
</script>

<div class="flex h-full flex-col">
    <DeviantArtFilters />

    <div class="flex-1 overflow-y-auto p-3" bind:this={scrollContainer}>
        {#if getIsSearching() && getResults().length === 0}
            <LoadingState message="Searching DeviantArt…" />
        {:else if getSearchError() && getResults().length === 0}
            <EmptyState
                title="Search failed"
                body={getSearchError()}
                actionLabel="Retry"
                onaction={search}
            />
        {:else if getResults().length === 0}
            <EmptyState
                title={getQuery() ? 'No matches found' : 'No walls to show'}
                body={getQuery()
                    ? `Nothing on DeviantArt matches the tag "${getQuery()}". Try a different tag.`
                    : 'Browse DeviantArt by tag — enter a tag such as "cyberpunk" to begin.'}
                actionLabel={getQuery() ? 'Clear tag' : undefined}
                onaction={getQuery() ? () => setQuery('') : undefined}
            >
                {#snippet icon()}
                    <SearchIcon size="h-12 w-12" strokeWidth={1.5} />
                {/snippet}
            </EmptyState>
        {:else}
            <div
                class="text-fg-dimmed mb-2 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5 text-[10px]"
            >
                <span class="text-fg-secondary text-[11px]"
                    >Tag “{getQuery()}”</span
                >
                <span class="tabular-nums"
                    >{getTotal().toLocaleString()} results</span
                >
                {#if authors.length > 0}
                    <span class="w-full truncate" title={authors.join(', ')}>
                        by {authors.slice(0, 3).join(', ')}{authors.length > 3
                            ? ` +${authors.length - 3} more`
                            : ''}
                    </span>
                {/if}
            </div>

            <WallpaperGrid
                wallpapers={getResults() as unknown as wallhaven.WallpaperInfo[]}
            />

            <div bind:this={sentinel} class="h-1 w-full"></div>

            {#if getIsLoadingMore()}
                <div class="flex h-12 items-center justify-center">
                    <span class="text-fg-dimmed text-[12px]"
                        >Loading more...</span
                    >
                </div>
            {:else if getSearchError()}
                <div
                    class="text-fg-dimmed flex items-center justify-center gap-3 py-3 text-[11px]"
                >
                    <span>{getSearchError()}</span>
                    <button
                        class="text-accent hover:text-accent-hover"
                        onclick={loadMore}>Retry</button
                    >
                </div>
            {:else if !getHasMore()}
                <div class="flex h-12 items-center justify-center">
                    <span class="text-fg-dimmed text-[12px]"
                        >End of results</span
                    >
                </div>
            {/if}
        {/if}
    </div>
</div>
