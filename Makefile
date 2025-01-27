# ====================================================================================
# Setup Project

PROJECT_NAME := crossplane-provider-cloudfoundry
PROJECT_REPO := github.tools.sap/cloud-orchestration/$(PROJECT_NAME)


export TERRAFORM_VERSION := 1.5.4

export TERRAFORM_PROVIDER_SOURCE := cloudfoundry-community/cloudfoundry
export TERRAFORM_PROVIDER_REPO := https://github.com/cloudfoundry-community/terraform-provider-cloudfoundry
export TERRAFORM_PROVIDER_VERSION := 0.51.2
export TERRAFORM_PROVIDER_DOWNLOAD_NAME := terraform-provider-cloudfoundry
export TERRAFORM_NATIVE_PROVIDER_BINARY := terraform-provider-cloudfoundry_0.51.2_linux_arm64
export TERRAFORM_DOCS_PATH := docs/resources

PLATFORMS ?= linux_amd64 linux_arm64

# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_REQUIRED_VERSION ?= 1.19
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider $(GO_PROJECT)/cmd/generator
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
GOLANGCILINT_VERSION ?= 1.53.3
-include build/makelib/golang.mk

# kind-related versions
KIND_VERSION ?= v0.22.0
KIND_NODE_IMAGE_TAG ?= v1.29.2

# Setup Kubernetes tools

KIND_VERSION = v0.15.0
UP_VERSION = v0.14.0
UP_CHANNEL = stable
UPTEST_VERSION = v0.2.1
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images
DOCKER_REGISTRY ?= crossplane
IMAGES = $(PROJECT_NAME) $(PROJECT_NAME)-controller
-include build/makelib/image.mk



export UUT_CONFIG = $(BUILD_REGISTRY)/$(subst crossplane-,crossplane/,$(PROJECT_NAME)):$(VERSION)
export UUT_CONTROLLER=$(BUILD_REGISTRY)/$(subst crossplane-,crossplane/,$(PROJECT_NAME))-controller:$(VERSION)
export E2E_IMAGES = {"crossplane/provider-cloudfoundry":"$(UUT_CONFIG)","crossplane/provider-cloudfoundry-controller":"$(UUT_CONTROLLER)"}

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.crossplane-provider-cloudfoundry-controller: do.build.images

# NOTE(hasheddan): we ensure up is installed prior to running platform-specific
# build steps in parallel to avoid encountering an installation race condition.
build.init: $(UP)

# ====================================================================================
# Fallthrough

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# ====================================================================================
# Setup Terraform for fetching provider schema
TERRAFORM := $(TOOLS_HOST_DIR)/terraform-$(TERRAFORM_VERSION)
TERRAFORM_WORKDIR := $(WORK_DIR)/terraform
TERRAFORM_PROVIDER_SCHEMA := config/schema.json

terraform.buildvars: common.buildvars
	@echo TERRAFORM_VERSION=$(TERRAFORM_VERSION)
	@echo TERRAFORM_PROVIDER_SOURCE=$(TERRAFORM_PROVIDER_SOURCE)
	@echo TERRAFORM_PROVIDER_REPO=$(TERRAFORM_PROVIDER_REPO)
	@echo TERRAFORM_PROVIDER_VERSION=$(TERRAFORM_PROVIDER_VERSION)
	@echo TERRAFORM_PROVIDER_DOWNLOAD_NAME=$(TERRAFORM_PROVIDER_DOWNLOAD_NAME)
	@echo TERRAFORM_NATIVE_PROVIDER_BINARY=$(TERRAFORM_NATIVE_PROVIDER_BINARY)
	@echo TERRAFORM_DOCS_PATH=$(TERRAFORM_DOCS_PATH)
	@echo TERRAFORM=$(TERRAFORM)
	@echo TERRAFORM_WORKDIR=$(TERRAFORM_WORKDIR)
	@echo TERRAFORM_PROVIDER_SCHEMA=$(TERRAFORM_PROVIDER_SCHEMA)


$(TERRAFORM):
	@$(INFO) installing terraform $(HOSTOS)-$(HOSTARCH)
	@mkdir -p $(TOOLS_HOST_DIR)/tmp-terraform
	@curl -fsSL https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_$(SAFEHOST_PLATFORM).zip -o $(TOOLS_HOST_DIR)/tmp-terraform/terraform.zip
	@unzip $(TOOLS_HOST_DIR)/tmp-terraform/terraform.zip -d $(TOOLS_HOST_DIR)/tmp-terraform
	@mv $(TOOLS_HOST_DIR)/tmp-terraform/terraform $(TERRAFORM)
	@rm -fr $(TOOLS_HOST_DIR)/tmp-terraform
	@$(OK) installing terraform $(HOSTOS)-$(HOSTARCH)

$(TERRAFORM_PROVIDER_SCHEMA): $(TERRAFORM)
	@$(INFO) generating provider schema for $(TERRAFORM_PROVIDER_SOURCE) $(TERRAFORM_PROVIDER_VERSION)
	@mkdir -p $(TERRAFORM_WORKDIR)
	@echo '{"terraform":[{"required_providers":[{"provider":{"source":"'"$(TERRAFORM_PROVIDER_SOURCE)"'","version":"'"$(TERRAFORM_PROVIDER_VERSION)"'"}}],"required_version":"'"$(TERRAFORM_VERSION)"'"}]}' > $(TERRAFORM_WORKDIR)/main.tf.json
	@$(TERRAFORM) -chdir=$(TERRAFORM_WORKDIR) init > $(TERRAFORM_WORKDIR)/terraform-logs.txt 2>&1
	@$(TERRAFORM) -chdir=$(TERRAFORM_WORKDIR) providers schema -json=true > $(TERRAFORM_PROVIDER_SCHEMA) 2>> $(TERRAFORM_WORKDIR)/terraform-logs.txt
	@$(OK) generating provider schema for $(TERRAFORM_PROVIDER_SOURCE) $(TERRAFORM_PROVIDER_VERSION)

pull-docs:
	@$(INFO) pull-docs called
	@if [ ! -d "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)" ]; then \
  		mkdir -p "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)" && \
		git clone -c advice.detachedHead=false --depth 1 --filter=blob:none --branch "v$(TERRAFORM_PROVIDER_VERSION)" --sparse "$(TERRAFORM_PROVIDER_REPO)" "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)"; \
	fi
	@git -C "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)" sparse-checkout set "$(TERRAFORM_DOCS_PATH)"

generate.init: $(TERRAFORM_PROVIDER_SCHEMA) pull-docs

.PHONY: $(TERRAFORM_PROVIDER_SCHEMA) pull-docs terraform.buildvars
# ====================================================================================
# Targets

# NOTE: the build submodule currently overrides XDG_CACHE_HOME in order to
# force the Helm 3 to use the .work/helm directory. This causes Go on Linux
# machines to use that directory as the build cache as well. We should adjust
# this behavior in the build submodule because it is also causing Linux users
# to duplicate their build cache, but for now we just make it easier to identify
# its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_ | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml


dev-debug: dev-clean $(KIND) $(KUBECTL)
	@$(INFO) Creating kind cluster
	@$(KIND) create cluster --name=$(PROJECT_NAME)-dev
	@$(KUBECTL) cluster-info --context kind-$(PROJECT_NAME)-dev
	@$(INFO) Installing Crossplane CRDs
	@$(KUBECTL) apply --server-side -k https://github.com/crossplane/crossplane//cluster?ref=master
	@$(INFO) Installing Provider CloudFoundry CRDs
	@$(KUBECTL) apply -R -f package/crds
	@$(INFO) Creating crossplane-system namespace
	@$(KUBECTL) create ns crossplane-system
	@$(INFO) Creating provider config and secret
	@$(KUBECTL) apply -R -f examples/providerconfig

dev-clean: $(KIND) $(KUBECTL)
	@$(INFO) Creating kind cluster
	@$(KIND) delete cluster --name=$(PROJECT_NAME)-dev


# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	UPBOUND_CONTEXT="local" $(GO_OUT_DIR)/provider --debug
# ====================================================================================
# Demo

demo-cluster: demo-clean $(KIND) $(KUBECTL) $(HELM3)
	@$(INFO) Creating kind cluster
	@$(KIND) create cluster --name=$(PROJECT_NAME)-demo
	@$(KUBECTL) cluster-info --context kind-$(PROJECT_NAME)-demo
	@$(INFO) Installing Crossplane
	@$(HELM3) repo add crossplane-stable https://charts.crossplane.io/stable
	@$(HELM3) repo update
	@$(KUBECTL) create ns crossplane-system
	@$(HELM3) install crossplane --namespace crossplane-system crossplane-stable/crossplane --set args='{--enable-composition-revisions}'

demo-install: $(KIND) $(KUBECTL)
	@$(INFO) Creating Orchestration-Registry, CIS, SA Secrets
	@$(KUBECTL) apply -R -f xdemo/secret
	@$(INFO) Installing Provider btp-account
	@$(KUBECTL) apply -f xdemo/btp-account/install.yaml
	sleep 10
	@$(INFO) Config Provider btp-account
	@$(KUBECTL) apply -R -f xdemo/btp-account/providerconfig
	@$(INFO) Installing CloudFoundry CRDs
	@$(KUBECTL) apply -R -f package/crds
	@$(INFO) Config RBAC locally for CloudFoundry CRDs
	@$(KUBECTL) apply -R -f xdemo/cloudfoundry/rbac
	@$(INFO) Install compostions
	@$(KUBECTL) apply -R -f xdemo/composition/xrds

demo-clean: $(KIND)
	@$(INFO) Deleting kind cluster
	@$(KIND) delete cluster --name=$(PROJECT_NAME)-demo

# ====================================================================================
# End to End Testing
CROSSPLANE_NAMESPACE = upbound-system
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

uptest: $(UPTEST) $(KUBECTL) $(KUTTL)
	@$(INFO) running automated tests
	@KUBECTL=$(KUBECTL) KUTTL=$(KUTTL) $(UPTEST) e2e "${UPTEST_EXAMPLE_LIST}" --setup-script=cluster/test/setup.sh || $(FAIL)
	@$(OK) running automated tests

local-deploy: build controlplane.up local.xpkg.deploy.provider.$(PROJECT_NAME)
	@$(INFO) running locally built provider
	@$(KUBECTL) wait provider.pkg $(PROJECT_NAME) --for condition=Healthy --timeout 5m
	@$(KUBECTL) -n upbound-system wait --for=condition=Available deployment --all --timeout=5m
	@$(OK) running locally built provider

e2e: local-deploy uptest

.PHONY: cobertura submodules fallthrough run crds.clean dev-debug dev-clean demo-cluster demo-install demo-clean demo-debug

.PHONY: test-acceptance
test-acceptance: $(KIND) $(HELM3) build generate-test-crs
	@$(INFO) running integration tests
	@$(INFO) Skipping long running tests
	@echo UUT_CONFIG=$$UUT_CONFIG
	@echo UUT_CONTROLLER=$$UUT_CONTROLLER
	@echo "E2E_IMAGES=$$E2E_IMAGES"
	go test -v  $(PROJECT_REPO)/test/e2e -tags=e2e -short -count=1 -test.v -timeout 40m
	@$(OK) integration tests passed

.PHONY:generate-test-crs
generate-test-crs:
	@$(INFO) generating crs
	find test/e2e/crs -type f -name "*.yaml" -exec sh -c '\
    	for template; do \
    		envsubst < "$$template" > "$${template}.tmp" && mv "$${template}.tmp" "$$template"; \
    	done' sh {} +
	@$(OK) crs generated
# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    cobertura             Generate a coverage report for cobertura applying exclusions on generated files.
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special
