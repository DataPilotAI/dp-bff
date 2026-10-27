DP_FE_VERSION ?= v0.1.0
UPSTREAM ?= http://localhost:8080
VITE_URL ?= http://localhost:5173

.PHONY: test build local docker

test:
	go vet ./...
	go test -race ./...

build:
	go build -trimpath -o bin/dp-bff ./cmd/dp-bff

# Local only: the BFF in front of the Vite server (hot reload). Run `npm run local` in dp-fe first.
local:
	DP_BFF_LOCAL_FRONTEND_URL=$(VITE_URL) DP_BFF_UPSTREAM_URL=$(UPSTREAM) go run ./cmd/dp-bff

# Image bundling the dp-fe release for DP_FE_VERSION (a pushed tag). Needs GH_TOKEN with read access to dp-fe.
docker:
	docker build --build-arg DP_FE_VERSION=$(DP_FE_VERSION) --secret id=github_token,env=GH_TOKEN -t dp-bff:$(DP_FE_VERSION) .
