GOLANG          := golang:1.22.0
ALPINE          := alpine:latest
KIND            := kindest/node:v1.30.0

KIND_CLUSTER    := nyx-cluster
NAMESPACE       := nyx-system
APP             := nyx
BASE_IMAGE_NAME := nyx/service
SERVICE_NAME    := nyx-api
VERSION         := 0.0.1
SERVICE_IMAGE   := $(BASE_IMAGE_NAME)/$(SERVICE_NAME):$(VERSION)
METRICS_IMAGE   := $(BASE_IMAGE_NAME)/$(SERVICE_NAME)-metrics:$(VERSION)

LINTER_VERSION = v1.52.1
LINTER_URL = https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh

GET_LINT_CMD = "curl -sSfL $(LINTER_URL) | sh -s -- -b $(go env GOPATH)/bin $(LINTER_VERSION)"

RED = \033[0;34m
GREEN = \033[0;32m
BLUE = \033[0;34m
COLOR_END = \033[0;39m

TEST_LIMIT = 500s

build-app:
	@echo "$(BLUE)» building application binary... $(COLOR_END)"
	@CGO_ENABLED=1 go build -a -o bin/$(APP) ./cmd/nyx
	@echo "Binary successfully built"

run-app:
	@./bin/${APP} -hostname=localhost:4001

.PHONY: test
test:
	go test -v ./internal/... -timeout $(TEST_LIMIT)

.PHONY: lint
lint:
	@echo "$(GREEN) Linting repository Go code...$(COLOR_END)"
	@if ! command -v golangci-lint &> /dev/null; \
	then \
    	echo "golangci-lint command could not be found...."; \
		echo "\nTo install, please run $(GREEN)  $(GET_LINT_CMD) $(COLOR_END)"; \
		echo "\nBuild instructions can be found at: https://golangci-lint.run/usage/install/."; \
    	exit 1; \
	fi

	@golangci-lint run

gosec:
	@echo "$(GREEN) Running security scan with gosec...$(COLOR_END)"
	gosec -exclude G104,G304 ./...

# ==============================================================================
# Load testing

client-rand-load-test:
	@echo "$(BLUE)» run client rand load tests... $(COLOR_END)"
	@CGO_ENABLED=1 go build -a -o bin/rand ./cmd/client/rand
	@echo "Binary successfully built"
	@./bin/rand -num-ops 100000 -num-workers 10

curl-set:
	curl -X POST "localhost:4001/api/set" -H "Authorization: Bearer 123"  -H "Content-Type: application/json" -d '{"key": "foo", "value": "bar", "exp": 3600}'

# ==============================================================================
# Running from within docker

.PHONY: docker-build
docker-build:
	@echo "$(GREEN) Building docker image...$(COLOR_END)"
	@docker build -t $(SERVICE_IMAGE) . --build-arg BUILD_REF=$(VERSION) --build-arg BUILD_DATE=`date -u +"%Y-%m-%dT%H:%M:%SZ"`

.PHONY: docker-run
docker-run:
	@echo "$(GREEN) Running docker image...$(COLOR_END)"
	@ docker run -p 8080:8080 -p 7300:7300 -it $(SERVICE_IMAGE)

# ==============================================================================
# Running from within k8s/kind
dev-up:
	kind create cluster \
		--image $(KIND) \
		--name $(KIND_CLUSTER)

	kubectl create namespace $(NAMESPACE)
	kind load docker-image $(SERVICE_IMAGE) --name $(KIND_CLUSTER)

dev-install-helm:
	helm install $(APP) deploy/nyx --namespace $(NAMESPACE)

dev-down:
	kind delete cluster --name $(KIND_CLUSTER)

dev-remove-helm:
	helm uninstall $(APP) --namespace $(NAMESPACE)

dev-status:
	kubectl get nodes -o wide
	kubectl get svc -o wide
	kubectl get pods -o wide --watch --all-namespaces

dev-load:
	kind load docker-image $(SERVICE_IMAGE) --name $(KIND_CLUSTER)

dev-restart:
	kubectl rollout restart deployment $(APP) --namespace=$(NAMESPACE)

dev-update:
	all dev-load dev-restart

dev-update-apply:
	all dev-load dev-apply

dev-logs:
	kubectl logs --namespace=$(NAMESPACE) -l app=$(APP) --all-containers=true -f --tail=100 --max-log-requests=6

# ==============================================================================
# Modules support

tidy:
	go mod tidy
	go mod vendor

deps-reset:
	git checkout -- go.mod
	go mod tidy
	go mod vendor
