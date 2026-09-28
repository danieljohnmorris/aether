# DeviantArt

Use DeviantArt as a wallpaper source. It is the strongest catalog for
art-grade illustrations and wallpapers that never reach wallpaper sites.

## Setup

DeviantArt requires OAuth client credentials. Aether uses the
**client-credentials** flow — no browser sign-in, no refresh token:

1. Register an application at [deviantart.com/developers](https://www.deviantart.com/developers/).
2. Open the `DeviantArt` tab, click `More`, and paste the client ID and client
   secret. They are stored locally at `~/.config/aether/deviantart.json`.

Aether exchanges the pair for an access token at
`https://www.deviantart.com/oauth2/token` (grant type `client_credentials`,
scope `browse`) and caches it in memory until it expires, refreshing when the
API answers 401.

## Searching

DeviantArt browsing is tag-based: type a tag such as `cyberpunk` and search.
Pagination follows the API's offset model (`limit` capped at 50 by
DeviantArt), and the result count comes from the API's estimate.

DeviantArt filters mature content itself for client-credentials tokens, so
`mature_content` is not sent — entries the API returns as mature are labelled
`mature` on the hit. Preview cards load through Aether, and `Use` downloads the
full-size deviation.

## Storage

- Credentials: `~/.config/aether/deviantart.json`
- Previews: `~/.cache/aether/deviantart-thumbs/`
- Downloads: `~/.local/share/aether/wallpapers/`

## CLI

```bash
aether --search-deviantart cyberpunk --limit 24
aether --deviantart-thumbs cyberpunk --limit 48 --json

# Credentials may also come from the flags
aether --search-deviantart cyberpunk --client-id ID --client-secret SECRET
```

## Rate Limits

The client-credentials token is shared by every search in one process, so a
long session performs a single token exchange. DeviantArt applies its own
per-token quotas; a rejected token surfaces as
`DeviantArt rejected the credentials (HTTP 401)` and the next search
re-authenticates.
