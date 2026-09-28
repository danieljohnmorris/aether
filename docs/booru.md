# Booru Catalogs

Use Danbooru-family image boards as a wallpaper source. These catalogs are
illustration-heavy, tag-driven, and need no API key for safe browsing — useful
for content that wallhaven does not carry.

## Supported Catalogs

| Site | Endpoint family | Credentials | Notes |
|------|-----------------|-------------|-------|
| `safebooru` | danbooru (`safebooru.donmai.us`) | not needed | SFW mirror; the default |
| `danbooru` | danbooru (`danbooru.donmai.us`) | optional `api_key` + `login` | raises the tag limit |
| `konachan` | moebooru (`konachan.net`) | not needed | no anonymous tag limit |
| `gelbooru` | gelbooru (`gelbooru.com`) | **required** `api_key` + `user_id` | [`gelbooru.com/index.php?page=api`](https://gelbooru.com/index.php?page=api) |

## Tag Budget

Safebooru and danbooru cap anonymous searches at **2 tags**. Aether spends that
budget on your query plus the filters it adds (`width:>=`, `height:>=`, and the
rating tag when `Safe` is on), so `cyberpunk` + a 4K filter + `Safe` is already
three tags. Either drop a filter or add credentials — the error names the site
and the budget.

Examples that fit the anonymous budget:

```
cyberpunk width:>=3840
cityscape rating:g
```

## Searching

1. Open the `Booru` tab.
2. Pick a catalog, type tags, and press Search.
3. `Min width` applies a resolution floor; `Safe` restricts to the site's safe
   rating; advanced options hold the page size and credentials.

Preview cards load through Aether rather than the webview, because the image
CDNs reject browser user agents. `Use` downloads the full-size image, sets it as
the wallpaper, and opens the editor; `Wallpaper only` applies it without
changing the palette; `Visit` opens the post page.

## Storage

- Credentials/config: `~/.config/aether/booru.json`
- Previews: `~/.cache/aether/booru-thumbs/`
- Downloads: `~/.local/share/aether/wallpapers/`

## CLI

```bash
# Metadata only
aether --search-booru "cyberpunk" --site safebooru --min-width 3840

# Search + cache preview thumbnails
aether --booru-thumbs "cyberpunk" --site konachan --limit 24 --json

# Gelbooru needs credentials, on the command line or in booru.json
aether --search-booru "cyberpunk" --site gelbooru --api-key KEY --user-id ID
```
