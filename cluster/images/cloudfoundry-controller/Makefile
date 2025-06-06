# ====================================================================================
# Setup Project

PLATFORMS := linux_amd64 linux_arm64

include ../../../build/makelib/common.mk

# ====================================================================================
#  Options
DOCKER_REGISTRY ?= crossplane
IMAGE = $(BUILD_REGISTRY)/crossplane/provider-cloudfoundry-controller
OSBASEIMAGE = scratch

include ../../../build/makelib/image.mk

# ====================================================================================
# Targets

.PHONY: img.build
img.build:
	@$(INFO) docker build $(IMAGE)
	$(MAKE) img.build.shared
	@$(OK) docker build $(IMAGE)

img.publish:
	@$(INFO) Skipping image publish for $(IMAGE)
	@echo Publish is deferred to xpkg machinery
	@$(OK) Image publish skipped for $(IMAGE)

img.build.shared:
	echo $(IMAGE_TEMP_DIR)
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp $(OUTPUT_DIR)/bin/$(OS)_$(ARCH)/provider $(IMAGE_TEMP_DIR) || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|BASEIMAGE|$(OSBASEIMAGE)|g' Dockerfile || $(FAIL)
	@docker build $(BUILD_ARGS) \
		--build-arg GIT_COMMIT="${COMMIT_HASH}" \
		--build-arg BUILD_TIMESTAMP="$(TIME_LONG)" \
		-t $(IMAGE) \
		-t $(IMAGE):$(VERSION) \
		$(IMAGE_TEMP_DIR) || $(FAIL)

img.promote:
	@$(INFO) Skipping image promotion from $(FROM_IMAGE) to $(TO_IMAGE)
	@echo Promote is deferred to xpkg machinery
	@$(OK) Image promotion skipped for $(FROM_IMAGE) to $(TO_IMAGE)
