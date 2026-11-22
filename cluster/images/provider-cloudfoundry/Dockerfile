FROM gcr.io/distroless/static-debian13@sha256:4c2cd1dee442c9a99924a378c7c126fac8fef14771579ef567c3f2bdd7a5122a

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/provider /usr/local/bin/crossplane-cloudfoundry-provider

USER 65532

WORKDIR /usr/local/bin

ENTRYPOINT ["crossplane-cloudfoundry-provider"]
