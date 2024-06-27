APP_NAME = nyx

LINTER_VERSION = v1.52.1
LINTER_URL = https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh

GET_LINT_CMD = "curl -sSfL $(LINTER_URL) | sh -s -- -b $(go env GOPATH)/bin $(LINTER_VERSION)"

RED = \033[0;34m
GREEN = \033[0;32m
BLUE = \033[0;34m
COLOR_END = \033[0;39m

TEST_LIMIT = 500s

run-app:
	@echo "$(BLUE)» building application binary... $(COLOR_END)"
	@CGO_ENABLED=1 go build -a -o bin/$(APP_NAME) ./cmd/nyx
	@echo "Binary successfully built"
	@./bin/${APP_NAME}

.PHONY: test
test:
	go test ./internal/... -timeout $(TEST_LIMIT)

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
	gosec ./...

tidy:
	go mod tidy
	go mod vendor

deps-reset:
	git checkout -- go.mod
	go mod tidy
	go mod vendor

client-load-test:
	@echo "$(BLUE)» run client load tests... $(COLOR_END)"
	@CGO_ENABLED=1 go build -a -o bin/client ./cmd/client
	@echo "Binary successfully built"
	@./bin/client -num-ops 100000 -num-workers 10

#curl-set:
#	curl -X POST "localhost:4001/api/set" -H "Authorization: Bearer 123"  -H "Content-Type: application/json" -d '{"key": "foo", "value": "bar", "exp": 3600}'
