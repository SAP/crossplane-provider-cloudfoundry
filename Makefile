# ====================================================================================
# Setup Project
BASE_NAME := cloudfoundry
PROJECT_NAME := crossplane-provider-$(BASE_NAME)
PROJECT_REPO := github.com/SAP/$(PROJECT_NAME)


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

GO_REQUIRED_VERSION ?= 1.23
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
GOLANGCILINT_VERSION ?= 1.64.5
-include build/makelib/golang.mk

# kind-related versions
KIND_VERSION ?= v0.26.0
KIND_NODE_IMAGE_TAG ?= v1.32.0

# Setup Kubernetes tools

KIND_VERSION = v0.22.0
UP_VERSION = v0.31.0
UP_CHANNEL = stable
UPTEST_VERSION = v0.11.1
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images
DOCKER_REGISTRY ?= crossplane
IMAGES = $(BASE_NAME) $(BASE_NAME)-controller

-include build/makelib/image.mk



export UUT_CONFIG = $(BUILD_REGISTRY)/$(subst crossplane-,crossplane/,$(PROJECT_NAME)):$(VERSION)
export UUT_CONTROLLER = $(BUILD_REGISTRY)/$(subst crossplane-,crossplane/,$(PROJECT_NAME))-controller:$(VERSION)
export E2E_IMAGES = {"package":"$(UUT_CONFIG)","controller":"$(UUT_CONTROLLER)"}

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
# ====================================================================================

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

# Updated End to End Testing following BTP Provider

.PHONY: test-acceptance
test-acceptance:  $(KIND) $(HELM3) build
	@$(INFO) running integration tests
	@$(INFO) Skipping long running tests
	@echo UUT_CONFIG=$$UUT_CONFIG
	@echo UUT_CONTROLLER=$$UUT_CONTROLLER
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
# Upgrade Tests
# ====================================================================================

# Upgrade test directory
UPGRADE_TEST_DIR := test/upgrade
UPGRADE_TEST_CRS_DIR := $(UPGRADE_TEST_DIR)/testdata/baseCrs
UPGRADE_TEST_OUTPUT_LOG := test-upgrade-output.log

# If UPGRADE_TEST_CRS_TAG is not set, use UPGRADE_TEST_FROM_TAG as default
UPGRADE_TEST_CRS_TAG ?= $(UPGRADE_TEST_FROM_TAG)

.PHONY: check-upgrade-test-vars
check-upgrade-test-vars: ## Verify required upgrade test environment variables
	@test -n "$(UPGRADE_TEST_FROM_TAG)" || { echo "❌ Set UPGRADE_TEST_FROM_TAG"; exit 1; }
	@test -n "$(UPGRADE_TEST_TO_TAG)" || { echo "❌ Set UPGRADE_TEST_TO_TAG"; exit 1; }
	@$(OK) required upgrade test environment variables are set

.PHONY: build-upgrade-test-images
build-upgrade-test-images: ## Build local images if testing with 'local' tag
	@if [ "$(UPGRADE_TEST_FROM_TAG)" = "local" ] || [ "$(UPGRADE_TEST_TO_TAG)" = "local" ]; then \
		$(INFO) "Building local images for upgrade test"; \
		$(MAKE) build VERSION=local; \
		$(INFO) "Tagging images with full registry path"; \
		docker tag $(BUILD_REGISTRY)/provider-$(BASE_NAME):local ghcr.io/sap/crossplane-provider-cloudfoundry/crossplane/provider-cloudfoundry:local || true; \
		docker tag $(BUILD_REGISTRY)/provider-$(BASE_NAME)-controller:local ghcr.io/sap/crossplane-provider-cloudfoundry/crossplane/provider-cloudfoundry-controller:local || true; \
		$(OK) "Built and tagged local images"; \
	fi

.PHONY: test-upgrade-compile
test-upgrade-compile: ## Verify upgrade tests compile
	@$(INFO) compiling upgrade tests
	@cd $(UPGRADE_TEST_DIR) && go test -c -tags=upgrade -o /dev/null
	@$(OK) upgrade tests compile successfully

.PHONY: test-upgrade
test-upgrade: $(KIND) check-upgrade-test-vars build-upgrade-test-images ## Run upgrade tests
	@$(INFO) running upgrade tests from $(UPGRADE_TEST_FROM_TAG) to $(UPGRADE_TEST_TO_TAG)
	@cd $(UPGRADE_TEST_DIR) && go test -v -tags=upgrade -timeout=45m ./... 2>&1 | tee ../../$(UPGRADE_TEST_OUTPUT_LOG)
	@echo "========== Upgrade Test Summary =========="
	@grep -E "PASS|FAIL|ok " $(UPGRADE_TEST_OUTPUT_LOG) | tail -5
	@case `tail -n 1 $(UPGRADE_TEST_OUTPUT_LOG)` in \
		*FAIL*) echo "❌ Upgrade test failed"; exit 1 ;; \
		*ok*) echo "✅ Upgrade tests passed"; $(OK) upgrade tests passed ;; \
		*) echo "⚠️  Could not determine test result"; exit 1 ;; \
	esac

.PHONY: test-upgrade-prepare-crs
test-upgrade-prepare-crs: ## Prepare CRs from CRS_TAG version
	@$(INFO) preparing CRs from $(UPGRADE_TEST_CRS_TAG)
	@test -n "$(UPGRADE_TEST_CRS_TAG)" || { echo "❌ Set UPGRADE_TEST_CRS_TAG or UPGRADE_TEST_FROM_TAG"; exit 1; }
	@if [ "$(UPGRADE_TEST_CRS_TAG)" = "local" ]; then \
		$(OK) "Using local CRs from $(UPGRADE_TEST_CRS_DIR)/ (CRS_TAG is 'local')"; \
	else \
		$(INFO) "Checking out CRs from tag $(UPGRADE_TEST_CRS_TAG)"; \
		rm -rf $(UPGRADE_TEST_CRS_DIR)/*; \
		mkdir -p $(UPGRADE_TEST_CRS_DIR); \
		if git ls-tree -r $(UPGRADE_TEST_CRS_TAG) --name-only | grep -q "^$(UPGRADE_TEST_CRS_DIR)/"; then \
			$(INFO) "✅ Found $(UPGRADE_TEST_CRS_DIR)/ in $(UPGRADE_TEST_CRS_TAG)"; \
			git archive $(UPGRADE_TEST_CRS_TAG) $(UPGRADE_TEST_CRS_DIR)/ | tar -x --strip-components=2 -C $(UPGRADE_TEST_CRS_DIR)/; \
			$(OK) "Copied all CRs from $(UPGRADE_TEST_CRS_DIR)/"; \
		else \
			$(INFO) "⚠️  $(UPGRADE_TEST_CRS_DIR)/ not found, using hardcoded e2e paths"; \
			git show $(UPGRADE_TEST_CRS_TAG):test/e2e/crs/orgspace/import.yaml > $(UPGRADE_TEST_CRS_DIR)/import.yaml 2>/dev/null || \
				{ echo "❌ Could not find import.yaml in $(UPGRADE_TEST_CRS_TAG)"; exit 1; }; \
			git show $(UPGRADE_TEST_CRS_TAG):test/e2e/crs/orgspace/space.yaml > $(UPGRADE_TEST_CRS_DIR)/space.yaml 2>/dev/null || \
				{ echo "❌ Could not find space.yaml in $(UPGRADE_TEST_CRS_TAG)"; exit 1; }; \
			$(OK) "Copied e2e CRs to $(UPGRADE_TEST_CRS_DIR)/"; \
		fi; \
	fi

.PHONY: test-upgrade-with-version-crs
test-upgrade-with-version-crs: $(KIND) check-upgrade-test-vars build-upgrade-test-images test-upgrade-prepare-crs ## Run upgrade tests with FROM version CRs
	@$(INFO) running upgrade tests from $(UPGRADE_TEST_FROM_TAG) to $(UPGRADE_TEST_TO_TAG)
	@cd $(UPGRADE_TEST_DIR) && go test -v -tags=upgrade -timeout=45m ./... 2>&1 | tee ../../$(UPGRADE_TEST_OUTPUT_LOG)
	@echo "========== Upgrade Test Summary =========="
	@grep -E "PASS|FAIL|ok " $(UPGRADE_TEST_OUTPUT_LOG) | tail -5
	@case `tail -n 1 $(UPGRADE_TEST_OUTPUT_LOG)` in \
		*FAIL*) echo "❌ Upgrade test failed"; exit 1; ;; \
		*ok*) echo "✅ Upgrade tests passed"; $(OK) upgrade tests passed; ;; \
		*) echo "⚠️  Could not determine test result"; exit 1; ;; \
	esac

.PHONY: test-upgrade-debug
test-upgrade-debug: $(KIND) check-upgrade-test-vars build-upgrade-test-images test-upgrade-prepare-crs ## Run upgrade tests with debugger
	@$(INFO) running upgrade tests with debugger
	@cd $(UPGRADE_TEST_DIR) && dlv test -tags=upgrade . --listen=:2345 --headless=true --api-version=2 --build-flags="-tags=upgrade" -- -test.v -test.timeout 45m 2>&1 | tee ../../$(UPGRADE_TEST_OUTPUT_LOG)
	@echo "========== Upgrade Test Summary =========="
	@grep -E "PASS|FAIL|ok " $(UPGRADE_TEST_OUTPUT_LOG) | tail -5


.PHONY: test-upgrade-restore-crs
test-upgrade-restore-crs: ## Restore $(UPGRADE_TEST_CRS_DIR)/ to current version
	@$(INFO) restoring $(UPGRADE_TEST_CRS_DIR)/ 
	@git checkout $(UPGRADE_TEST_CRS_DIR)/
	@$(OK) CRs restored

.PHONY: test-upgrade-clean
test-upgrade-clean: $(KIND) ## Clean upgrade test artifacts
	@$(INFO) cleaning upgrade test artifacts
	@$(KIND) get clusters 2>/dev/null | grep e2e | xargs -r -n1 $(KIND) delete cluster --name || true
	@rm -rf $(UPGRADE_TEST_DIR)/logs/
	@rm -f $(UPGRADE_TEST_OUTPUT_LOG)
	@$(OK) cleanup complete

.PHONY: test-upgrade-help
test-upgrade-help: ## Show upgrade test usage examples
	@$(INFO) ""
	@$(INFO) "Upgrade Test Examples:"
	@$(INFO) "======================"
	@$(INFO) ""
	@$(INFO) "  1. Test between two releases:"
	@$(INFO) "     export UPGRADE_TEST_FROM_TAG=v0.3.2"
	@$(INFO) "     export UPGRADE_TEST_TO_TAG=v0.4.0"
	@$(INFO) "     make test-upgrade"
	@$(INFO) ""
	@$(INFO) "  2. Test local changes (v0.3.2 -> your code):"
	@$(INFO) "     export UPGRADE_TEST_FROM_TAG=v0.3.2"
	@$(INFO) "     export UPGRADE_TEST_TO_TAG=local"
	@$(INFO) "     make test-upgrade"
	@$(INFO) ""
	@$(INFO) "  3. Manual upgrade test (no CR checkout):"
	@$(INFO) "     export UPGRADE_TEST_FROM_TAG=v0.3.2"
	@$(INFO) "     export UPGRADE_TEST_TO_TAG=v0.4.0"
	@$(INFO) "     make test-upgrade"
	@$(INFO) "     Note: Uses current $(UPGRADE_TEST_CRS_DIR) (may fail if incompatible)"
	@$(INFO) ""
	@$(INFO) "  4. Clean up test artifacts:"
	@$(INFO) "     make test-upgrade-clean"
	@$(INFO) ""
	@$(INFO) "  5. Restore CRs after version checkout:"
	@$(INFO) "     make test-upgrade-restore-crs"
	@$(INFO) ""
	@$(INFO) "Required Environment Variables:"
	@$(INFO) "  CF_EMAIL, CF_USERNAME, CF_PASSWORD, CF_ENDPOINT"
	@$(INFO) "  UPGRADE_TEST_FROM_TAG, UPGRADE_TEST_TO_TAG"
	@$(INFO) ""
	@$(INFO) "Optional Environment Variables:"
	@$(INFO) "  UPGRADE_TEST_CRS_PATH (default: $(UPGRADE_TEST_CRS_DIR))"
	@$(INFO) "  UPGRADE_TEST_VERIFY_TIMEOUT (default: 30 minutes)"
	@$(INFO) "  UPGRADE_TEST_WAIT_FOR_PAUSE (default: 1 minute)"
	@$(INFO) ""
	@$(INFO) "How CRS Checkout Works (test-upgrade-with-version-crs):"
	@$(INFO) "========================================================"
	@$(INFO) "  1. If FROM_TAG is 'local': Uses current $(UPGRADE_TEST_CRS_DIR)/"
	@$(INFO) "  2. If FROM_TAG has $(UPGRADE_TEST_CRS_DIR)/: Copies entire directory"
	@$(INFO) ""
	@$(INFO) ""
	@$(INFO) "⚠️  IMPORTANT NOTES:"
	@$(INFO) "  - test-upgrade-with-version-crs OVERWRITES $(UPGRADE_TEST_CRS_DIR)/"
	@$(INFO) "  - test-upgrade-restore-crs will to restore your files"
	@$(INFO) "  - E2E CRs (fallback) may have complex dependencies - test might fail"
	@$(INFO) ""

	
# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    cobertura             Generate a coverage report for cobertura applying exclusions on generated files.
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

Upgrade Testing:
    test-upgrade                   Run upgrade tests (requires env vars)
    test-upgrade-with-version-crs  Run upgrade tests with auto CR checkout
    test-upgrade-compile           Verify upgrade tests compile
    test-upgrade-debug             Run upgrade tests with debugger
    test-upgrade-prepare-crs       Prepare CRs from CRS_TAG version
    test-upgrade-restore-crs       Restore test/upgrade/crs/ to current version
    test-upgrade-clean             Clean up upgrade test artifacts
    test-upgrade-help              Show detailed upgrade test usage
    check-upgrade-test-vars        Verify required environment variables
    build-upgrade-test-images      Build local images if needed
endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special

PUBLISH_IMAGES ?= crossplane/provider-cloudfoundry crossplane/provider-cloudfoundry-controller

.PHONY: publish
publish:
	@$(INFO) "Publishing images $(PUBLISH_IMAGES) to $(DOCKER_REGISTRY)"
	@for image in $(PUBLISH_IMAGES); do \
		echo "Publishing image $(DOCKER_REGISTRY)/$${image}:$(VERSION)"; \
		docker push $(DOCKER_REGISTRY)/$${image}:$(VERSION); \
	done
	@$(OK) "Publishing images $(PUBLISH_IMAGES) to $(DOCKER_REGISTRY)"