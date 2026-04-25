# Migration notes — `github.com/docker/docker` → `github.com/moby/moby/client`

**Status**: deferred. The migration has been scoped but not executed. This
note exists so the next person picking it up doesn't have to re-discover
the surface area.

## Why migrate

`github.com/docker/docker` is the deprecated Go import path. Moby has
reorganized:

- `github.com/moby/moby/v2` — root module, currently `v2.0.0-betaN`.
- `github.com/moby/moby/api` — split-out API types (currently `v1.54.x`).
- `github.com/moby/moby/client` — the standalone client SDK (currently
  `v0.4.x`).

Tagged Docker Engine releases like `docker-v29.4.1` ship under the
`v2` umbrella; the Go module proxies don't recognize the `docker-v…`
prefix as semver, which is why a naive `go get
github.com/docker/docker@v29.4.1+incompatible` fails. This is a Go-tooling
artefact, not a "v29 isn't released" thing.

The 2 remaining Dependabot alerts on `github.com/docker/docker` (high
+ medium) are advisories about the daemon binary, not the SDK we
embed. Migrating to the new client paths makes Dependabot stop
flagging us and gets us off the deprecated import.

## Current pin

`agents/docker-agent/go.mod` currently uses
`github.com/docker/docker v28.5.2+incompatible`. The SDK works at this
version. The migration is a scope question, not a correctness question.

## What changes — module imports

| Old import | New import |
|---|---|
| `github.com/docker/docker/client` | `github.com/moby/moby/client` |
| `github.com/docker/docker/api/types/container` | `github.com/moby/moby/api/types/container` |
| `github.com/docker/docker/api/types/network` | `github.com/moby/moby/api/types/network` |
| `github.com/docker/docker/api/types/image` | `github.com/moby/moby/api/types/image` |
| `github.com/docker/docker/api/types/mount` | `github.com/moby/moby/api/types/mount` |
| `github.com/docker/docker/api/types/filters` | **gone** — replaced by `client.Filters` map type |
| `github.com/docker/docker/api/types` (umbrella) | `github.com/moby/moby/api/types` |

## What changes — call signatures

The new SDK is uniformly `Method(ctx, ...path, OptionsStruct) (ResultStruct, error)`.
Almost every call site needs a small wrapper rewrite.

### `ContainerCreate`

Old:
```go
resp, err := cli.ContainerCreate(ctx, config, hostCfg, netCfg, nil, name)
// resp.ID
```

New:
```go
resp, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
    Config:           config,
    HostConfig:       hostCfg,
    NetworkingConfig: netCfg,
    Name:             name,
})
// resp.ID
```

### `ContainerInspect`

Old:
```go
inspect, err := cli.ContainerInspect(ctx, id)
// inspect.NetworkSettings, inspect.State, etc.
```

New:
```go
inspect, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
// inspect.Container.NetworkSettings, etc.  (note the .Container hop)
```

### `ServiceInspectWithRaw`

Old:
```go
service, raw, err := cli.ServiceInspectWithRaw(ctx, id, types.ServiceInspectOptions{})
```

New:
```go
res, err := cli.ServiceInspect(ctx, id, client.ServiceInspectOptions{})
service := res.Service
raw := res.Raw
```

### `ServiceUpdate`

Old:
```go
resp, err := cli.ServiceUpdate(ctx, id, version, spec, types.ServiceUpdateOptions{})
```

New:
```go
res, err := cli.ServiceUpdate(ctx, id, client.ServiceUpdateOptions{
    Version: version,
    Spec:    spec,
    // (encoded registry auth, etc. moved into the options too)
})
```

### `TaskList` filters

Old:
```go
tasks, err := cli.TaskList(ctx, types.TaskListOptions{
    Filters: filters.NewArgs(filters.Arg("id", taskID)),
})
```

New:
```go
res, err := cli.TaskList(ctx, client.TaskListOptions{
    Filters: client.Filters{}.Add("id", taskID),
})
tasks := res.Tasks
```

### Container/Network/Image option types

In v28.5 these moved from `types.ContainerListOptions` etc. to
`container.ListOptions`. In the new client they move again to
`client.ContainerListOptions`. We did the first half in PR #257; the
second half is what this migration finishes.

## Call-site inventory (as of `cbbac0e`)

```
agents/docker-agent/
├── main.go                                    (client construction, Ping, Close)
├── agent_handlers.go                          (1 ContainerInspect)
├── agent_docker_operations.go                 (~9 calls)
│   NetworkList, NetworkCreate, ContainerCreate,
│   ImageInspectWithRaw, ImagePull, ContainerStart,
│   ContainerInspect, ContainerStop, ContainerRemove, ContainerList
└── internal/leaderelection/
    ├── swarm_backend.go                       (Info, ContainerInspect,
    │                                            TaskList, ServiceInspect ×4,
    │                                            ServiceUpdate ×3)
    └── swarm_backend_test.go                  (tests covering the above)
```

Total surface: ~20–25 call sites. Mostly mechanical, a few that need
the `.Container` / `.Service` hop on the new Result struct.

## Suggested approach

1. New branch from `main`.
2. Drop `github.com/docker/docker` from `agents/docker-agent/go.mod`.
3. Add `github.com/moby/moby/client` and `github.com/moby/moby/api`
   explicitly. **Do not** add `github.com/moby/moby` (the umbrella
   module) — it provides the same package paths and creates ambiguous
   imports.
4. Search-and-replace the import paths above (`perl -i` is fine).
5. Build and let the compiler walk you through the call-site fixes.
6. Replace `filters.NewArgs / filters.Arg` with `client.Filters{}.Add`.
7. Run `go test ./...` — the existing leaderelection tests cover the
   swarm path; rerun with `-race` once green.
8. Verify the build also works for the K8s agent (it doesn't use the
   docker SDK, but a `go build ./...` from the repo root is a cheap
   regression check).

Allow ~1.5–3 h of careful work for this. Worth doing before adding new
SDK call sites.
