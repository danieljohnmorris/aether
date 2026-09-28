// DeviantArt search state
// Loads saved credentials from ~/.config/aether/deviantart.json (via Go),
// persists changes back to the config file. DeviantArt searches go through
// their OAuth2 client_credentials API, so both a client ID and secret are
// required before any search can run.

import type {wallpapersrc} from '../../../wailsjs/go/models';

// Card shape handed to the shared WallpaperGrid — mirrors the provider-agnostic
// contract the grid/card consume (downloads, favourites, previews and "Visit"
// are handled by the card itself).
export interface DevArtItem {
    id: string;
    path: string;
    resolution: string;
    file_size: number;
    thumbs: {large: string};
    pageUrl: string;
    provider: string;
    site: string;
    title: string;
    author?: string;
}

const DEFAULT_LIMIT = 24;
const MIN_LIMIT = 1;
const MAX_LIMIT = 50;

function clampLimit(value: number): number {
    if (!Number.isFinite(value)) return DEFAULT_LIMIT;
    return Math.min(MAX_LIMIT, Math.max(MIN_LIMIT, Math.floor(value)));
}

let query = $state('');
let limit = $state(DEFAULT_LIMIT);
let clientId = $state('');
let clientSecret = $state('');

let results = $state<DevArtItem[]>([]);
let totalResults = $state(0);
let totalPages = $state(0);
let page = $state(1);
let isSearching = $state(false);
let isLoadingMore = $state(false);
let searchError = $state('');
let hasSearched = false;
let searchRequest = 0;

// Load config from ~/.config/aether/deviantart.json on startup
const configReady = loadConfigFromFile();

async function loadConfigFromFile() {
    try {
        const {GetDeviantArtConfig, SetDeviantArtCredentials} = await import(
            '../../../wailsjs/go/main/App'
        );
        const config = await GetDeviantArtConfig();
        if (config) {
            if (typeof config.clientId === 'string') clientId = config.clientId;
            if (typeof config.clientSecret === 'string') {
                clientSecret = config.clientSecret;
            }
            if (typeof config.limit === 'number') {
                limit = clampLimit(config.limit);
            }
            if (clientId || clientSecret) {
                await SetDeviantArtCredentials(clientId, clientSecret);
            }
        }
    } catch {}
}

function persist() {
    import('../../../wailsjs/go/main/App')
        .then(({SaveDeviantArtConfig}) => {
            return SaveDeviantArtConfig({clientId, clientSecret, limit});
        })
        .catch(() => {});
}

function syncCredentials() {
    import('../../../wailsjs/go/main/App')
        .then(({SetDeviantArtCredentials}) => {
            return SetDeviantArtCredentials(clientId, clientSecret);
        })
        .catch(() => {});
}

// --- Getters ---
export function getQuery(): string {
    return query;
}
export function getLimit(): number {
    return limit;
}
export function getClientId(): string {
    return clientId;
}
export function getClientSecret(): string {
    return clientSecret;
}
export function getConfigured(): boolean {
    return clientId.trim() !== '' && clientSecret.trim() !== '';
}
export function getResults(): DevArtItem[] {
    return results;
}
export function getTotal(): number {
    return totalResults;
}
export function getIsSearching(): boolean {
    return isSearching;
}
export function getIsLoadingMore(): boolean {
    return isLoadingMore;
}
export function getHasMore(): boolean {
    if (totalPages > 0) return page < totalPages;
    // Totals unknown (meta.total === 0): assume another page while the last
    // fetch returned a full page.
    return results.length >= limit;
}
export function getSearchError(): string {
    return searchError;
}

// --- Setters (all persist) ---
export function setQuery(q: string): void {
    query = q;
}
export function setLimit(v: number): void {
    limit = clampLimit(v);
    persist();
}
export function setClientId(v: string): void {
    clientId = v;
    persist();
    syncCredentials();
}
export function setClientSecret(v: string): void {
    clientSecret = v;
    persist();
    syncCredentials();
}

// --- Actions ---
export async function initializeSearch(): Promise<void> {
    await configReady;
    if (hasSearched) return;
    if (!getConfigured()) return;
    if (query.trim() === '') return;
    search();
}

export function search(): void {
    void doSearch(false);
}

export function loadMore(): void {
    if (isSearching || isLoadingMore) return;
    if (!getHasMore()) return;
    void doSearch(true);
}

function mapHit(hit: wallpapersrc.Hit): DevArtItem {
    return {
        id: hit.id,
        path: hit.imageUrl,
        resolution: hit.resolution,
        file_size: hit.fileSize,
        thumbs: {large: hit.previewUrl},
        pageUrl: hit.pageUrl,
        provider: 'deviantart',
        site: hit.site,
        title: hit.title || hit.id,
        author: hit.author,
    };
}

async function doSearch(append: boolean): Promise<void> {
    if (!getConfigured()) {
        searchError = 'Add your DeviantArt client ID and secret to search.';
        return;
    }

    const request = ++searchRequest;
    hasSearched = true;
    searchError = '';
    if (append) isLoadingMore = true;
    else {
        isSearching = true;
        isLoadingMore = false;
        results = [];
        totalPages = 0;
        totalResults = 0;
        page = 1;
    }

    const params = {
        query,
        site: '',
        page: append ? page + 1 : 1,
        limit,
        minWidth: 0,
        minHeight: 0,
        safeOnly: true,
    } as unknown as wallpapersrc.SearchQuery;

    try {
        await configReady;
        if (request !== searchRequest) return;
        const {SearchDeviantArt} = await import(
            '../../../wailsjs/go/main/App'
        );
        if (request !== searchRequest) return;
        const result = await SearchDeviantArt(params);
        if (request !== searchRequest) return;
        const data = (result.data || []).map(mapHit);
        if (append) {
            const seen = new Set(results.map(r => r.id));
            results = [...results, ...data.filter(r => !seen.has(r.id))];
        } else {
            results = data;
        }
        totalPages = result.meta?.last_page || 0;
        totalResults = result.meta?.total || 0;
        page = params.page;
    } catch (e) {
        if (request !== searchRequest) return;
        console.error('DeviantArt search failed:', e);
        searchError =
            e instanceof Error ? e.message : String(e || 'Search failed.');
    } finally {
        if (request === searchRequest) {
            isLoadingMore = false;
            isSearching = false;
        }
    }
}
