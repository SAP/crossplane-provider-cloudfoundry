# ====================================================================================
# Setup Project
BASE_NAME := cloudfoundry
PROJECT_NAME := provider-$(BASE_NAME)
PROJECT_REPO := github.com/SAP/crossplane-$(PROJECT_NAME)


PLATFORMS ?= linux_amd64 linux_arm64
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse HEAD)
$(info VERSION is $(VERSION))

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

GO_REQUIRED_VERSION ?= 1.22
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
GOLANGCILINT_VERSION ?= 1.62.2
-include build/makelib/golang.mk

# Setup Kubernetes tools
KIND_VERSION = v0.22.0
UP_VERSION = v0.31.0
UP_CHANNEL = stable
UPTEST_VERSION = v0.11.1
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images
IMAGES = provider-cloudfoundry
-include build/makelib/imagelight.mk



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
# Setup XPKG

# XPKG_REG_ORGS ?= xpkg.upbound.io/crossplane-contrib index.docker.io/crossplanecontrib
# NOTE(hasheddan): skip promoting on xpkg.upbound.io as channel tags are
# inferred.
# XPKG_REG_ORGS_NO_PROMOTE ?= xpkg.upbound.io/crossplane-contrib
XPKGS ?= provider-cloudfoundry
XPKG_REG_ORGS ?= ghcr.io/sap/crossplane-provider-cloudfoundry/crossplane
-include build/makelib/xpkg.mk

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.crossplane-provider-cloudfoundry: do.build.images

# NOTE: the build submodule currently overrides XDG_CACHE_HOME in order to
# force the Helm 3 to use the .work/helm directory. This causes Go on Linux
# machines to use that directory as the build cache as well. We should adjust
# this behavior in the build submodule because it is also causing Linux users
# to duplicate their build cache, but for now we just make it easier to identify
# its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# ====================================================================================
# Targets

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_ | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml


dev-debug: dev-clean $(KIND) $(KUBECTL) $(HELM3)
	@$(INFO) Creating kind cluster
	@$(KIND) create cluster --name=$(PROJECT_NAME)-dev
	@$(KUBECTL) cluster-info --context kind-$(PROJECT_NAME)-dev
	@$(INFO) Installing Crossplane
	@$(HELM3) repo add crossplane-stable https://charts.crossplane.io/stable
	@$(HELM3) repo update
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
# End to End Testing
CROSSPLANE_NAMESPACE = upbound-system
-include build/makelib/local.xpkg.mk
CROSSPLANE_ARGS = '--enable-usages'
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

# Updated End to End Testing following BTP Provider

export E2E_REUSE_CLUSTER = local-dev
export E2E_CLUSTER_NAME = local-dev

.PHONY: test-acceptance
test-acceptance: local-deploy $(KUBECTL)
	@echo "Creating crossplane-system namespace"
	@$(KUBECTL) create namespace crossplane-system
	@$(INFO) running integration tests
	@$(INFO) Skipping long running tests
	@$(INFO) ${E2E_IMAGES}
	@echo "E2E_IMAGES=$$E2E_IMAGES"
	go test -v  $(PROJECT_REPO)/test/e2e -tags=e2e -short -count=1 -test.v -run '$(testFilter)' 2>&1 | tee test-output.log
	@echo "===========Test Summary==========="
	@grep -E "PASS|FAIL" test-output.log
	@case `tail -n 1 test-output.log` in \
		*FAIL*) echo "❌ Error: Test failed"; exit 1 ;; \
		*) echo "✅ All tests passed"; $(OK) integration tests passed ;; \
	esac
.PHONY: cobertura submodules fallthrough run crds.clean dev-debug dev-clean demo-cluster demo-install demo-clean demo-debug

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
