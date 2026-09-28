// GridWallpaper is the structural shape the shared wallpaper card and grid
// accept. Wallhaven results, the external providers (booru catalogs,
// DeviantArt) and any future source map into it, so every provider renders
// with the same card: `path` is the full-size image URL (or local path),
// `thumbs.large` the remote preview, and `provider` the source identifier the
// backend dispatches downloads on ("wallhaven" when absent).
export type GridWallpaper = {
    id: string;
    path: string;
    resolution: string;
    file_size?: number;
    thumbs?: Record<string, string>;
    /** Remote page for the "Visit" action; defaults to the wallhaven page. */
    pageUrl?: string;
    /** Provider identifier used by the backend: booru, deviantart, wallhaven. */
    provider?: string;
    /** Provider-local catalog name, e.g. safebooru. */
    site?: string;
    title?: string;
    author?: string;
};
