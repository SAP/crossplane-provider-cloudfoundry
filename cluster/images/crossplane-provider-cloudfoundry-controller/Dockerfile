ARG BUILDENV=upbound


FROM scratch AS upbound
ADD provider /provider
ARG BASEDIR

FROM scratch AS piper
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG BASEDIR=cluster/images/crossplane-provider-cloudfoundry-controller/

ARG TERRAFORM_VERSION=1.5.4
ARG TERRAFORM_PROVIDER_VERSION=0.51.2

ARG TERRAFORM_PROVIDER_SOURCE=cloudfoundry-community/cloudfoundry
ARG TERRAFORM_PROVIDER_DOWNLOAD_NAME=terraform-provider-cloudfoundry
ARG TERRAFORM_NATIVE_PROVIDER_BINARY=${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_VERSION}_linux_arm64

ADD provider-$TARGETOS.$TARGETARCH /provider

FROM ${BUILDENV} AS buildenv

FROM alpine:3.19.1@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b
RUN apk --no-cache add ca-certificates bash
ARG TARGETOS=linux
ARG TARGETARCH=amd64


COPY --from=buildenv /provider /usr/local/bin/crossplane-provider-cloudfoundry

ENV USER_ID=65532

# Setup Terraform environment

## Provider-dependent configuration
ARG TERRAFORM_VERSION
ARG TERRAFORM_PROVIDER_SOURCE
ARG TERRAFORM_PROVIDER_VERSION
ARG TERRAFORM_PROVIDER_DOWNLOAD_NAME
ARG TERRAFORM_NATIVE_PROVIDER_BINARY

## End of - Provider-dependent configuration

ARG BUILD_TIMESTAMP
ARG GIT_COMMIT
ENV PLUGIN_DIR /terraform/provider-mirror/registry.terraform.io/${TERRAFORM_PROVIDER_SOURCE}/${TERRAFORM_PROVIDER_VERSION}/${TARGETOS}_${TARGETARCH}
ENV TF_CLI_CONFIG_FILE /terraform/.terraformrc
ENV TF_FORK 0

RUN mkdir -p ${PLUGIN_DIR}

ADD https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_${TARGETOS}_${TARGETARCH}.zip /tmp
ADD https://github.com/cloudfoundry-community/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}/releases/download/v${TERRAFORM_PROVIDER_VERSION}/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_PROVIDER_VERSION}_${TARGETOS}_${TARGETARCH}.zip /tmp
#ADD https://releases.hashicorp.com/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}/${TERRAFORM_PROVIDER_VERSION}/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_PROVIDER_VERSION}_${TARGETOS}_${TARGETARCH}.zip /tmp
ADD ${BASEDIR}terraformrc.hcl ${TF_CLI_CONFIG_FILE}

RUN unzip /tmp/terraform_${TERRAFORM_VERSION}_${TARGETOS}_${TARGETARCH}.zip -d /usr/local/bin \
  && chmod +x /usr/local/bin/terraform \
  && rm /tmp/terraform_${TERRAFORM_VERSION}_${TARGETOS}_${TARGETARCH}.zip \
  && unzip /tmp/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_PROVIDER_VERSION}_${TARGETOS}_${TARGETARCH}.zip -d ${PLUGIN_DIR} \
  && chmod +x ${PLUGIN_DIR}/* \
  && rm /tmp/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_PROVIDER_VERSION}_${TARGETOS}_${TARGETARCH}.zip \
  && chown -R ${USER_ID}:${USER_ID} /terraform
# End of - Setup Terraform environment

# Provider controller needs these environment variable at runtime
ENV TERRAFORM_VERSION ${TERRAFORM_VERSION}
ENV TERRAFORM_PROVIDER_SOURCE ${TERRAFORM_PROVIDER_SOURCE}
ENV TERRAFORM_PROVIDER_VERSION ${TERRAFORM_PROVIDER_VERSION}
ENV TERRAFORM_NATIVE_PROVIDER_PATH ${PLUGIN_DIR}/${TERRAFORM_NATIVE_PROVIDER_BINARY}
ENV TF_APPEND_USER_AGENT "crossplane-provider-cloudfoundry/${CROSSPLANE_PROVIDER_VERSION}"

USER ${USER_ID}
EXPOSE 8080

ENTRYPOINT ["crossplane-provider-cloudfoundry"]


LABEL org.opencontainers.image.title="crossplane-provider-cloudfoundry-controller" \
      org.opencontainers.image.description="Crossplane controller for CloudFoundry" \
      org.opencontainers.image.documentation="https://pages.github.tools.sap/cloud-orchestration/" \
      org.opencontainers.image.url="https://github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry" \
      org.opencontainers.image.source="https://github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry.git" \
      org.opencontainers.image.vendor="SAP SE" \
      org.opencontainers.image.base.name="docker.io/library/alpine:3.19.1" \
      org.opencontainers.image.base.digest="sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b" \
      org.opencontainers.image.revision="$GIT_COMMIT" \
      org.opencontainers.image.created="$BUILD_TIMESTAMP"
