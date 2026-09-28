// Booru search state (Safebooru / Danbooru / Konachan / Gelbooru).
// Loads config from ~/.config/aether/booru.json (via Go) and persists changes
// back to the config file. Results are mapped into the shared card shape so the
// generic wallpaper grid can render them.

import type {wallpapersrc} from '../../../wailsjs/go/models';

export type BooruItem = {
    id: string;
    path: string;
    resolution: string;
    file_size: number;
    thumbs: Record<string, string>;
    pageUrl: string;
    provider: string;
    site: string;
    title?: string;
    author?: string;
};

const SITES: {id: string; label: string}[] = [
    {id: 'safebooru', label: 'Safebooru (SFW)'},
    {id: 'danbooru', label: 'Danbooru'},
    {id: 'konachan', label: 'Konachan'},
    {id: 'gelbooru', label: 'Gelbooru (needs credentials)'},
];

let query = $state('');
let site = $state('safebooru');
let minWidth = $state('3840');
let safeOnly = $state(true);
let limit = $state(24);
let apiKey = $state('');
let userID = $state('');

let results = $state<BooruItem[]>([]);
let total = $state(0);
let page = $state(1);
let lastPage = $state(0);
let lastPageSize = $state(0);
let isSearching = $state(false);
let isLoadingMore = $state(false);
let searchError = $state('');
let hasSearched = false;
let searchRequest = 0;
let lastSearchParams: wallpapersrc.SearchQuery | null = null;

// Load config from ~/.config/aether/booru.json on startup
const configReady = loadConfigFromFile();

async function loadConfigFromFile() {
    try {
        const {GetBooruConfig, SetBooruCredentials} = await import(
            '../../../wailsjs/go/main/App'
        );
        const config = await GetBooruConfig();
        if (config) {
            if (config.site) site = config.site;
            if (config.minWidth !== undefined && config.minWidth !== null)
                minWidth = config.minWidth;
            if (typeof config.safeOnly === 'boolean') safeOnly = config.safeOnly;
            if (config.apiKey) apiKey = config.apiKey;
            if (config.userID) userID = config.userID;
            if (apiKey || userID) await SetBooruCredentials(apiKey, userID);
        }
    } catch {}
}

function persist() {
    import('../../../wailsjs/go/main/App')
        .then(({SaveBooruConfig}) => {
            return SaveBooruConfig({site, minWidth, safeOnly, apiKey, userID});
        })
        .catch(() => {});
}

function mapHit(hit: wallpapersrc.Hit): BooruItem {
    return {
        id: hit.id,
        path: hit.imageUrl,
        resolution: hit.resolution,
        file_size: hit.fileSize,
        thumbs: {large: hit.previewUrl},
        pageUrl: hit.pageUrl,
        provider: 'booru',
        site: hit.site,
        title: hit.title,
        author: hit.author,
    };
}

// --- Getters ---
export function getQuery(): string {
    return query;
}
export function getSite(): string {
    return site;
}
export function getMinWidth(): string {
    return minWidth;
}
export function getSafeOnly(): boolean {
    return safeOnly;
}
export function getLimit(): number {
    return limit;
}
export function getApiKey(): string {
    return apiKey;
}
export function getUserID(): string {
    return userID;
}
export function getResults(): BooruItem[] {
    return results;
}
export function getTotal(): number {
    return total;
}
export function getIsSearching(): boolean {
    return isSearching;
}
export function getIsLoadingMore(): boolean {
    return isLoadingMore;
}
export function getHasMore(): boolean {
    // The booru APIs don't report totals reliably: keep paging while the last
    // page came back full and the reported last page hasn't been reached.
    return lastPageSize >= limit && (lastPage === 0 || page < lastPage);
}
export function getSearchError(): string {
    return searchError;
}
export function getSites(): {id: string; label: string}[] {
    return SITES;
}

// --- Setters ---
export function setQuery(q: string): void {
    query = q;
}
export function setSite(s: string): void {
    site = s;
    persist();
}
export function setMinWidth(w: string): void {
    minWidth = w;
    persist();
}
export function toggleSafeOnly(): void {
    safeOnly = !safeOnly;
    persist();
}
export function setLimit(l: number): void {
    limit = l;
}
export function setApiKey(k: string): void {
    apiKey = k;
    persist();
    import('../../../wailsjs/go/main/App')
        .then(({SetBooruCredentials}) => {
            return SetBooruCredentials(apiKey, userID);
        })
        .catch(() => {});
}
export function setUserID(id: string): void {
    userID = id;
    persist();
    import('../../../wailsjs/go/main/App')
        .then(({SetBooruCredentials}) => {
            return SetBooruCredentials(apiKey, userID);
        })
        .catch(() => {});
}

// --- Actions ---
export async function initializeSearch(): Promise<void> {
    await configReady;
    // Never auto-search an empty query.
    if (query && !hasSearched) await doSearch(false);
}

export function search(): void {
    void doSearch(false);
}

export function loadMore(): void {
    if (isSearching || isLoadingMore) return;
    if (!getHasMore()) return;
    void doSearch(true);
}

async function doSearch(append: boolean): Promise<void> {
    const request = ++searchRequest;
    hasSearched = true;
    searchError = '';
    if (append) isLoadingMore = true;
    else {
        isSearching = true;
        isLoadingMore = false;
        results = [];
        total = 0;
        lastPage = 0;
        lastPageSize = 0;
        page = 1;
    }
    try {
        await configReady;
        if (request !== searchRequest) return;
        const params = (append && lastSearchParams
            ? {...lastSearchParams, page: page + 1}
            : {
                  query,
                  site,
                  page: 1,
                  limit,
                  minWidth: minWidth ? parseInt(minWidth, 10) : 0,
                  minHeight: 0,
                  safeOnly,
                  apiKey: apiKey || undefined,
                  userID: userID || undefined,
              }) as unknown as wallpapersrc.SearchQuery;
        const {SearchBooru} = await import('../../../wailsjs/go/main/App');
        if (request !== searchRequest) return;
        const result = await SearchBooru(params);
        if (request !== searchRequest) return;
        const data = result.data || [];
        if (append) {
            const seen = new Set(results.map(r => r.id));
            results = [
                ...results,
                ...data.map(mapHit).filter(r => !seen.has(r.id)),
            ];
        } else {
            results = data.map(mapHit);
        }
        total = result.meta?.total || 0;
        lastPage = result.meta?.last_page || 0;
        lastPageSize = data.length;
        page = params.page;
        lastSearchParams = params;
    } catch (e) {
        if (request !== searchRequest) return;
        console.error('Booru search failed:', e);
        searchError =
            'Could not reach the booru. Check your connection and filters, then retry.';
    } finally {
        if (request === searchRequest) {
            isLoadingMore = false;
            isSearching = false;
        }
    }
}
