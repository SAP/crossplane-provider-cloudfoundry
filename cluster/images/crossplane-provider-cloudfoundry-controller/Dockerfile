ARG BUILDENV=upbound


FROM scratch AS upbound
ADD provider /provider
ARG BASEDIR

FROM scratch AS piper
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG BASEDIR=cluster/images/crossplane-provider-cloudfoundry-controller/

ADD provider-$TARGETOS.$TARGETARCH /provider

FROM ${BUILDENV} AS buildenv

FROM alpine:3.20.3@sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a
RUN apk --no-cache add ca-certificates bash
ARG TARGETOS=linux
ARG TARGETARCH=amd64

COPY --from=buildenv /provider /usr/local/bin/crossplane-provider-cloudfoundry

ENV USER_ID=65532

ARG BUILD_TIMESTAMP
ARG GIT_COMMIT

USER ${USER_ID}
EXPOSE 8080

ENTRYPOINT ["crossplane-provider-cloudfoundry"]


LABEL org.opencontainers.image.title="crossplane-provider-cloudfoundry-controller" \
      org.opencontainers.image.description="Crossplane controller for CloudFoundry" \
      org.opencontainers.image.documentation="https://pages.github.com/SAP/" \
      org.opencontainers.image.url="https://github.com/SAP/crossplane-provider-cloudfoundry" \
      org.opencontainers.image.source="https://github.com/SAP/crossplane-provider-cloudfoundry.git" \
      org.opencontainers.image.vendor="SAP SE" \
      org.opencontainers.image.base.name="docker.io/library/alpine:3.20.3" \
      org.opencontainers.image.base.digest="sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a" \
      org.opencontainers.image.revision="$GIT_COMMIT" \
      org.opencontainers.image.created="$BUILD_TIMESTAMP"
