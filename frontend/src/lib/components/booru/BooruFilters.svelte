<script lang="ts">
    import {
        getQuery,
        setQuery,
        getSite,
        setSite,
        getSites,
        getMinWidth,
        setMinWidth,
        getSafeOnly,
        toggleSafeOnly,
        getLimit,
        setLimit,
        getApiKey,
        setApiKey,
        getUserID,
        setUserID,
        getTotal,
        search,
    } from '$lib/stores/booru.svelte';

    let showAdvanced = $state(false);

    function handleSubmit(e: Event) {
        e.preventDefault();
        search();
    }
</script>

<form class="bg-bg-secondary border-border border-b" onsubmit={handleSubmit}>
    <!-- Row 1: search + site + submit -->
    <div class="flex items-center gap-2 px-3 pb-1.5 pt-2.5">
        <input
            type="text"
            class="bg-bg-surface border-border text-fg-primary focus:border-border-focus min-w-[160px] flex-1 border px-2 py-1.5 text-[12px] outline-none"
            placeholder="Search booru tags… (e.g. cyberpunk)"
            value={getQuery()}
            oninput={e => setQuery(e.currentTarget.value)}
        />

        <select
            class="bg-bg-surface border-border text-fg-secondary focus:border-border-focus border px-2 py-1.5 text-[11px] outline-none"
            value={getSite()}
            onchange={e => setSite(e.currentTarget.value)}
            title="Source site"
        >
            {#each getSites() as s}
                <option value={s.id}>{s.label}</option>
            {/each}
        </select>

        <button
            type="submit"
            class="bg-accent hover:bg-accent-hover text-accent-fg px-3 py-1.5 text-[11px] font-medium transition-colors"
            >Search</button
        >
    </div>

    <!-- Row 2: min width + safe toggle + meta -->
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-3 py-2">
        <select
            class="bg-bg-surface border-border text-fg-dimmed hover:text-fg-secondary focus:border-border-focus border px-1.5 py-0.5 text-[10px] outline-none"
            value={getMinWidth()}
            onchange={e => setMinWidth(e.currentTarget.value)}
            title="Minimum width"
        >
            <option value="">Any width</option>
            <option value="1920">1920+</option>
            <option value="2560">2560+</option>
            <option value="3840">3840+</option>
            <option value="5120">5120+</option>
        </select>

        <span class="bg-border h-3.5 w-px"></span>

        <button
            type="button"
            class="border px-1.5 py-0.5 text-[10px] transition-colors
            {getSafeOnly()
                ? 'text-success border-success bg-success/10'
                : 'text-fg-dimmed border-border hover:text-fg-secondary'}"
            onclick={() => toggleSafeOnly()}
            aria-pressed={getSafeOnly()}
            title="Only safe-for-work results">Safe</button
        >

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
            class="border-border flex flex-wrap items-center gap-x-4 gap-y-2 border-t px-3 py-2.5"
        >
            <div class="flex items-center gap-1.5">
                <span class="text-fg-dimmed text-[10px]" title="Results per page"
                    >Limit</span
                >
                <input
                    type="number"
                    min="1"
                    max="100"
                    class="bg-bg-surface border-border text-fg-primary focus:border-border-focus w-[64px] border px-2 py-1 text-[11px] outline-none"
                    value={getLimit()}
                    oninput={e =>
                        setLimit(Number(e.currentTarget.value) || 1)}
                />
            </div>

            <div class="flex min-w-[180px] flex-1 items-center gap-2">
                <span
                    class="text-fg-dimmed shrink-0 text-[10px]"
                    title="Required by some sites and for larger result pages"
                >
                    API key
                </span>
                <input
                    type="password"
                    class="bg-bg-surface border-border text-fg-primary focus:border-border-focus min-w-0 flex-1 border px-2 py-1 text-[11px] outline-none"
                    placeholder="API key…"
                    value={getApiKey()}
                    oninput={e => setApiKey(e.currentTarget.value)}
                />
            </div>

            <div class="flex min-w-[180px] flex-1 items-center gap-2">
                <span
                    class="text-fg-dimmed shrink-0 text-[10px]"
                    title="Required by some sites (e.g. Gelbooru)"
                >
                    User ID
                </span>
                <input
                    type="text"
                    class="bg-bg-surface border-border text-fg-primary focus:border-border-focus min-w-0 flex-1 border px-2 py-1 text-[11px] outline-none"
                    placeholder="User ID…"
                    value={getUserID()}
                    oninput={e => setUserID(e.currentTarget.value)}
                />
            </div>
        </div>
    {/if}
</form>
