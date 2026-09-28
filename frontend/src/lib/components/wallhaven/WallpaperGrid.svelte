<script lang="ts">
    import WallpaperCard from './WallpaperCard.svelte';
    import ImagePreview from '$lib/components/shared/ImagePreview.svelte';
    import {getCardSize, CARD_MIN_WIDTH} from '$lib/stores/cardsize.svelte';
    import {getSourcePreview} from '$lib/stores/sourcepreviews.svelte';
    import type {GridWallpaper} from '$lib/types/wallpaper';

    let {wallpapers}: {wallpapers: GridWallpaper[]} = $props();
    let previewIndex = $state(-1);

    function getPreviewSrc(wp: GridWallpaper): string {
        // Prefer the provider preview already fetched through the backend:
        // some provider CDNs reject the webview's browser user agent, so the
        // remote URL would fail to load here.
        const providerPreview = wp.provider
            ? getSourcePreview(wp.provider, wp.thumbs?.large ?? '')
            : '';
        return (
            providerPreview ||
            wp.path ||
            wp.thumbs?.original ||
            wp.thumbs?.large ||
            ''
        );
    }
</script>

<div
    class="grid gap-3"
    style:grid-template-columns="repeat(auto-fill, minmax({CARD_MIN_WIDTH[
        getCardSize()
    ]}px, 1fr))"
>
    {#each wallpapers as wp, i (wp.id)}
        <WallpaperCard wallpaper={wp} onpreview={() => (previewIndex = i)} />
    {/each}
</div>

<ImagePreview
    src={previewIndex >= 0 ? getPreviewSrc(wallpapers[previewIndex]) : ''}
    alt={previewIndex >= 0 ? wallpapers[previewIndex].title || wallpapers[previewIndex].id : ''}
    open={previewIndex >= 0}
    onclose={() => (previewIndex = -1)}
    hasPrev={previewIndex > 0}
    hasNext={previewIndex < wallpapers.length - 1}
    onprev={() => previewIndex--}
    onnext={() => previewIndex++}
/>
