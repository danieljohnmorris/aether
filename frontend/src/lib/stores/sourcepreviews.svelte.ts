// Preview cache for the external wallpaper providers (booru catalogs,
// DeviantArt).
//
// Provider CDNs reject the webview's browser user agent (cdn.donmai.us answers
// 403 to a Mozilla UA), so previews are fetched through the Go backend, which
// returns a bounded, cached image as a data URL. Results are keyed by
// source + remote URL, so switching tabs or re-searching reuses the cache.

import {GetSourceThumbnail} from '../../../wailsjs/go/main/App';

let cache = $state<Record<string, string>>({});
let pending: Record<string, boolean> = {};

function cacheKey(source: string, url: string): string {
    return source + '|' + url;
}

// getSourcePreview returns the cached data URL, or '' when not loaded yet.
export function getSourcePreview(source: string, url: string): string {
    if (!source || !url) return '';
    return cache[cacheKey(source, url)] ?? '';
}

// isLoadingSourcePreview reports whether a fetch for this preview is in flight.
export function isLoadingSourcePreview(source: string, url: string): boolean {
    return pending[cacheKey(source, url)] === true;
}

// loadSourcePreview fetches a provider preview through the backend and caches
// the resulting data URL. A failed fetch returns '' and stays retryable.
export async function loadSourcePreview(
    source: string,
    url: string
): Promise<string> {
    if (!source || !url) return '';
    const key = cacheKey(source, url);
    if (cache[key]) return cache[key];
    if (pending[key]) return '';

    pending = {...pending, [key]: true};
    try {
        const result = await GetSourceThumbnail(source, url);
        if (!result?.dataURL) return '';
        cache = {...cache, [key]: result.dataURL};
        return result.dataURL;
    } catch {
        return '';
    } finally {
        const next = {...pending};
        delete next[key];
        pending = next;
    }
}
