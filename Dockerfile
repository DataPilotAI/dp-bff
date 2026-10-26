# syntax=docker/dockerfile:1

# Pinned frontend release, e.g. --build-arg DP_FE_VERSION=v0.1.0
ARG DP_FE_VERSION

FROM alpine:3.21 AS frontend
ARG DP_FE_VERSION
RUN apk add --no-cache github-cli
WORKDIR /download
# The token is a build secret (--secret id=github_token,...); it never lands in an image layer.
RUN --mount=type=secret,id=github_token \
    test -n "${DP_FE_VERSION}" \
    && GH_TOKEN="$(cat /run/secrets/github_token)" gh release download "${DP_FE_VERSION}" \
         --repo DataPilotAI/dp-fe --pattern "dp-fe-${DP_FE_VERSION}.tar.gz*" \
    && sha256sum -c "dp-fe-${DP_FE_VERSION}.tar.gz.sha256" \
    && mkdir /static \
    && tar -xzf "dp-fe-${DP_FE_VERSION}.tar.gz" -C /static

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dp-bff ./cmd/dp-bff

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /dp-bff /dp-bff
COPY --from=frontend /static /static
ENV DP_BFF_STATIC_DIR=/static
EXPOSE 8081
ENTRYPOINT ["/dp-bff"]
