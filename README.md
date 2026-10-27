# dp-bff

Backend-for-frontend for DataPilot AI. It serves the built `dp-fe` bundle and proxies `/api/` to the backend, so the browser talks to a single origin.

- Static files with SPA fallback (unknown client routes get `index.html`, unknown files get 404)
- Reverse proxy that does not buffer, so chat streams (SSE) arrive token by token
- Security headers (CSP, nosniff, frame and referrer policy, HSTS over TLS), request body limit, access log without query strings
- Standard library only, no third-party dependencies

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DP_BFF_UPSTREAM_URL` | required | Backend the `/api/` requests are forwarded to, for example `http://localhost:8080` |
| `DP_BFF_ADDR` | `:8081` | Listen address |
| `DP_BFF_STATIC_DIR` | `static` | Directory holding the unpacked frontend build; must contain `index.html` |
| `DP_BFF_MAX_BODY_MB` | `55` | Largest accepted request body (keep it above the frontend upload limit) |
| `DP_BFF_LOCAL_FRONTEND_URL` | unset | Local only: proxy the frontend to a local Vite server instead of serving `DP_BFF_STATIC_DIR` (loopback hosts only) |

The server refuses to start if `index.html` is missing. `GET /healthz` returns 204.

## Run locally (hot reload)

Run the Vite server in `dp-fe`, then start the BFF in local mode. The BFF serves everything on :8081, proxies pages and the HMR websocket to Vite, and still proxies `/api/` to the backend. The path is forwarded to the backend unchanged (`/api/datasets` stays `/api/datasets`).

```sh
# terminal 1, in dp-fe
npm run local

# terminal 2, in dp-bff
make local UPSTREAM=http://localhost:8080   # then open http://localhost:8081
```

Local mode turns the Content-Security-Policy off (Vite's dev client needs inline scripts), so it only accepts a `localhost` or loopback `DP_BFF_LOCAL_FRONTEND_URL`. Never set it in a deployed environment (dev, staging or production).

## Frontend version (deployed environments)

The frontend is pinned by tag. Releasing `dp-fe` (push a `v*` tag) publishes `dp-fe-<tag>.tar.gz` and a checksum; the image build downloads exactly that version and verifies the checksum.

```sh
GH_TOKEN=... make docker DP_FE_VERSION=v0.1.0
docker run --rm -p 8081:8081 -e DP_BFF_UPSTREAM_URL=http://host.docker.internal:8080 dp-bff:v0.1.0
```

`GH_TOKEN` is passed as a BuildKit secret and is not stored in the image. Upgrading the frontend means changing `DP_FE_VERSION` and rebuilding.

## Notes

- No read or write timeout is set on the server on purpose, so long uploads and streams are not cut off. Put a timeout at the edge if you need one.
- Authentication is not implemented. This is the place to add it (session cookie checked before the proxy), and the backend must still authorise every request itself.
- No response compression; do that at the edge or add it here later.
- Set HSTS at the TLS-terminating proxy; the server only sends it when it terminates TLS itself.
