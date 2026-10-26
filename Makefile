DP_FE_VERSION ?= v0.1.0
UPSTREAM ?= http://localhost:8080

.PHONY: test build fetch-fe run docker

test:
	go vet ./...
	go test -race ./...

build:
	go build -trimpath -o bin/dp-bff ./cmd/dp-bff

# Downloads the pinned frontend release into ./static (needs the gh CLI and access to dp-fe).
fetch-fe:
	rm -rf static && mkdir static
	gh release download $(DP_FE_VERSION) --repo DataPilotAI/dp-fe --pattern 'dp-fe-$(DP_FE_VERSION).tar.gz*' --dir static
	cd static && sha256sum -c dp-fe-$(DP_FE_VERSION).tar.gz.sha256 && tar -xzf dp-fe-$(DP_FE_VERSION).tar.gz && rm dp-fe-*

run:
	DP_BFF_UPSTREAM_URL=$(UPSTREAM) go run ./cmd/dp-bff

docker:
	docker build --build-arg DP_FE_VERSION=$(DP_FE_VERSION) --secret id=github_token,env=GH_TOKEN -t dp-bff:$(DP_FE_VERSION) .
