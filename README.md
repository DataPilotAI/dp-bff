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

The server refuses to start if `index.html` is missing. `GET /healthz` returns 204.

## Run locally

```sh
make fetch-fe DP_FE_VERSION=v0.1.0   # downloads the dp-fe release into ./static (needs gh)
make run UPSTREAM=http://localhost:8080
```

Then open http://localhost:8081. The path is forwarded to the upstream unchanged (`/api/datasets` stays `/api/datasets`).

## Frontend version

The frontend is pinned by tag. Releasing `dp-fe` (push a `v*` tag) publishes `dp-fe-<tag>.tar.gz` and a checksum; this repo downloads exactly that version and verifies the checksum.

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
