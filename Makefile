# the dep target fetches required dependencies
# it should be removed as soon as k8s.io/code-generator can be specified as a
# 'required' dependency in Gopkg.toml, and replaced by a call to dep ensure
# (see https://github.com/golang/dep/issues/1306)
.PHONY: dep
dep: KUBERNETES_VERSION=1.9.6
dep: KUBERNETES_CODE_GENERATOR_PKG=k8s.io/code-generator
dep: KUBERNETES_APIMACHINERY_PKG=k8s.io/apimachinery
dep:
	dep ensure
	go get -d $(KUBERNETES_CODE_GENERATOR_PKG)/...
	cd $(GOPATH)/src/$(KUBERNETES_CODE_GENERATOR_PKG) && \
		git fetch origin && \
		git checkout -f kubernetes-$(KUBERNETES_VERSION)
	go get -d $(KUBERNETES_APIMACHINERY_PKG)/...
	cd $(GOPATH)/src/$(KUBERNETES_APIMACHINERY_PKG) && \
		git fetch origin && \
		git checkout -f kubernetes-$(KUBERNETES_VERSION)

.PHONY: dev
dev: PROFILE?=minikube
dev:
	skaffold dev -p $(PROFILE)

.PHONY: run
run: PROFILE?=minikube
run:
	skaffold run -p $(PROFILE)

.PHONY: docker.operator
docker.operator: TAG?=$(shell git describe --dirty)
docker.operator: IMG?=quay.io/travelaudience/aerospike-operator
docker.operator:
	docker build -t $(IMG):$(TAG) -f ./Dockerfile .

.PHONY: docker.tools
docker.tools: TAG?=$(shell git describe --dirty)
docker.tools: IMG?=quay.io/travelaudience/aerospike-operator-tools
docker.tools:
	docker build -t $(IMG):$(TAG) -f ./Dockerfile.tools .

.PHONY: fmt
fmt:
	./hack/update-fmt.sh

.PHONY: gen
gen: export CODEGEN_PKG=../../../k8s.io/code-generator
gen:
	./hack/update-codegen.sh

.PHONY: test.unit
test.unit:
	go test -v ./cmd/... ./pkg/...

.PHONY: test.e2e
test.e2e: FLAKE_ATTEMPTS?=3
test.e2e: FOCUS?=
test.e2e: GCS_BUCKET_NAME?=
test.e2e: GCS_SECRET_NAME?=
test.e2e: TIMEOUT?=1800s
test.e2e:
	kubectl -n aerospike-operator exec aerospike-operator -- go test -v -timeout=$(TIMEOUT) ./test/e2e \
		-ginkgo.flakeAttempts=$(FLAKE_ATTEMPTS) \
		-ginkgo.focus=$(FOCUS) \
		-gcs-bucket-name=$(GCS_BUCKET_NAME) \
		-gcs-secret-name=$(GCS_SECRET_NAME) \
		$(EXTRA_FLAGS)
