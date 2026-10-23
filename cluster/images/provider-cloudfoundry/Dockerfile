FROM gcr.io/distroless/static-debian13@sha256:034df5c1737e8241f6ed6423a12e720ee0f6d6f088609ee172f98f6f2166ef5d

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/provider /usr/local/bin/crossplane-cloudfoundry-provider

USER 65532

WORKDIR /usr/local/bin

ENTRYPOINT ["crossplane-cloudfoundry-provider"]
