# Contributing

Thanks for taking a look. Issues and pull requests are welcome.

## Never commit a real backup

The most important rule in this repository. An etcd snapshot holds every Secret
in the cluster it came from, plus real namespace and hostname data. `.gitignore`
blocks `*.db`, `*.zip` and `*.tar.gz` for this reason, but check `git status`
before committing anyway. Downloads started from the web UI land in your working
directory, which is the usual way one of these sneaks in.

Use the generated fixture instead:

```sh
make demo    # writes ./demo-backup with a synthetic cluster
make run-demo
```

Screenshots in `docs/` must come from the demo backup, never from a real one.

## Getting set up

You need Go (the version in `go.mod`) and Node 24 or newer.

```sh
git clone https://github.com/mjovanovic0/openshift-etcd-backup-explorer
cd openshift-etcd-backup-explorer
make build        # builds the web UI, then the binary that embeds it
make test         # go test plus a TypeScript build
```

For UI work, `make dev` runs the Go API on `:8080` and Vite on `:5173` with
`/api` proxied, so the browser reloads as you edit.

Note that `go build ./...` on its own produces a binary with no web UI, because
the built assets are not committed. It will start and serve the API, and the UI
route explains what to run. `make build` is the full path.

## Running the tests

```sh
make test                  # uses ./backup if you have one, else the fixture
OEBE_FIXTURE=demo go test ./...   # the way CI runs, fixture only
```

If you have a real backup in `./backup`, the suite uses it, which catches
things a synthetic fixture cannot. CI only ever sees the generated one.

When you change how a stored object is read, add a case to
`internal/kube/meta_test.go`. Those tests round trip objects through the real
Kubernetes protobuf serializer, so they check the hand written parsers against
the exact bytes an API server writes. Two real bugs were found that way.

## Style

- Run `make fmt` and `make vet` before pushing. CI enforces `gofmt`.
- Comments explain *why*, not what the line already says. Skip a comment that
  only restates the code.
- Prose in comments, docs and UI copy is plain English: short sentences,
  no em dashes.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org),
  for example `feat: export a whole resource type as a zip` or
  `fix: keep object references when stripping generated fields`. The release
  notes are generated from them.

## Pull requests

Say what the change does and why. If it changes what the UI looks like, a
screenshot from the demo backup helps. Keep unrelated changes in separate pull
requests.

By contributing you agree that your work is licensed under the
[Apache License 2.0](LICENSE).
