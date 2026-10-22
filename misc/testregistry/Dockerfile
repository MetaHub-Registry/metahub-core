# syntax=docker/dockerfile:1.4
#
# Multi-arch test registry used by metalib's testcontainers helpers
# (pkg/upstream/test_helper.go -> SetupRegistry).
#
# The registry serves a small pre-seeded catalogue, so the tests do not depend on
# network access to public registries:
#
#   alpine                                     3.17.2, 3.17.3, manifest
#   cqnib/metahub-layers/hpc3/glow             1.5.0-{amd64,arm64}, 1.5.1-{amd64,arm64}
#   qnib-pub-containers/qnib/plain-featuretest 20220715-arm64-generic
#
# That data lives in quay.io/cqnib/testcontainer-registry:2023-05-31.1-amd64,
# which was only ever published for amd64 — on an arm64 agent the container
# started with `exec /entrypoint.sh: exec format error`.
#
# Registry data is architecture-neutral; only the distribution binary is not.
# So: lift /opt/registry out of the legacy amd64 image (the seed stage runs on
# the build host; COPY does not execute anything, so no emulation is involved)
# and drop it onto the multi-arch `registry` image, once per target arch.
#
# Build and publish:
#
#   docker buildx build \
#     --platform linux/amd64,linux/arm64 \
#     -t quay.io/cqnib/testcontainer-registry:2023-05-31.1 \
#     --push .

ARG REGISTRY_IMAGE=registry:2.8.3

FROM --platform=$BUILDPLATFORM quay.io/cqnib/testcontainer-registry:2023-05-31.1-amd64 AS seed

FROM ${REGISTRY_IMAGE}
COPY --from=seed /opt/registry /opt/registry
