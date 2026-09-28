<script lang="ts">
    import {
        getQuery,
        setQuery,
        getLimit,
        setLimit,
        getClientId,
        setClientId,
        getClientSecret,
        setClientSecret,
        getConfigured,
        getTotal,
        search,
    } from '$lib/stores/deviantart.svelte';
    import CardSizeToggle from '$lib/components/shared/CardSizeToggle.svelte';

    let showAdvanced = $state(false);

    function handleSubmit(e: Event) {
        e.preventDefault();
        search();
    }
</script>

<form class="bg-bg-secondary border-border border-b" onsubmit={handleSubmit}>
    <!-- Row 1: search + primary controls -->
    <div class="flex items-center gap-2 px-3 pb-1.5 pt-2.5">
        <input
            type="text"
            class="bg-bg-surface border-border text-fg-primary focus:border-border-focus min-w-[160px] flex-1 border px-2 py-1.5 text-[12px] outline-none"
            placeholder="Browse a tag… (e.g. cyberpunk)"
            value={getQuery()}
            oninput={e => setQuery(e.currentTarget.value)}
        />

        <select
            class="bg-bg-surface border-border text-fg-secondary focus:border-border-focus border px-2 py-1.5 text-[11px] outline-none"
            value={getLimit()}
            onchange={e => setLimit(Number(e.currentTarget.value))}
            title="Results per page"
        >
            <option value={12}>12 per page</option>
            <option value={24}>24 per page</option>
            <option value={48}>48 per page</option>
        </select>

        <button
            type="submit"
            class="bg-accent hover:bg-accent-hover text-accent-fg px-3 py-1.5 text-[11px] font-medium transition-colors"
            >Search</button
        >
    </div>

    <!-- Row 2: meta + advanced toggle -->
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-3 py-2">
        <CardSizeToggle />

        <div class="ml-auto flex items-center gap-3">
            {#if getTotal() > 0}
                <span class="text-fg-dimmed text-[10px] tabular-nums"
                    >{getTotal().toLocaleString()} results</span
                >
            {/if}
            <button
                type="button"
                class="hover:text-fg-secondary text-[10px] transition-colors {showAdvanced
                    ? 'text-accent'
                    : 'text-fg-dimmed'}"
                onclick={() => (showAdvanced = !showAdvanced)}
                aria-expanded={showAdvanced}
                >{showAdvanced ? 'Less' : 'More'}</button
            >
        </div>
    </div>

    {#if showAdvanced}
        <div
            class="border-border flex flex-col gap-2 border-t px-3 py-2.5"
        >
            <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
                <div class="flex min-w-[200px] flex-1 items-center gap-2">
                    <span class="text-fg-dimmed shrink-0 text-[10px]"
                        >Client ID</span
                    >
                    <input
                        type="text"
                        class="bg-bg-surface border-border text-fg-primary focus:border-border-focus min-w-0 flex-1 border px-2 py-1 text-[11px] outline-none"
                        placeholder="DeviantArt client ID…"
                        value={getClientId()}
                        oninput={e => setClientId(e.currentTarget.value)}
                    />
                </div>

                <div class="flex min-w-[200px] flex-1 items-center gap-2">
                    <span class="text-fg-dimmed shrink-0 text-[10px]"
                        >Client secret</span
                    >
                    <input
                        type="password"
                        class="bg-bg-surface border-border text-fg-primary focus:border-border-focus min-w-0 flex-1 border px-2 py-1 text-[11px] outline-none"
                        placeholder="DeviantArt client secret…"
                        value={getClientSecret()}
                        oninput={e => setClientSecret(e.currentTarget.value)}
                    />
                </div>
            </div>

            {#if !getConfigured()}
                <p class="text-fg-dimmed text-[10px]">
                    Add your DeviantArt client ID and secret to search.
                </p>
            {/if}
        </div>
    {/if}
</form>
