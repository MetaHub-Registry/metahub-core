# Multi-arch test registry

`metalib`'s testcontainers helpers (`pkg/upstream/test_helper.go`) start a
registry that already serves the fixture catalogue, so the tests do not depend
on network access to public registries:

| repository | tags |
|---|---|
| `alpine` | `3.17.2`, `3.17.3`, `manifest` |
| `cqnib/metahub-layers/hpc3/glow` | `1.5.0-{amd64,arm64}`, `1.5.1-{amd64,arm64}` |
| `qnib-pub-containers/qnib/plain-featuretest` | `20220715-arm64-generic` |

That catalogue used to ship only as `quay.io/cqnib/testcontainer-registry:2023-05-31.1-amd64`.
On an arm64 agent the container died immediately with
`exec /entrypoint.sh: exec format error`, failing `metahub-api_main_prod`'s Test stage.

Registry data is architecture-neutral; only the distribution binary is not.
`Dockerfile` therefore lifts `/opt/registry` out of the legacy amd64 image (the
seed stage is pinned to `$BUILDPLATFORM` and only does a `COPY`, so nothing is
emulated) and drops it onto the multi-arch `registry` image, once per target
arch.

Published as `quay.io/cqnib/testcontainer-registry:2023-05-31.1`
(`linux/amd64` + `linux/arm64`), which is what the helper uses. Rebuild and
publish with:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t quay.io/cqnib/testcontainer-registry:2023-05-31.1 \
  --push .
```
