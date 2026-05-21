# Tooling With Docker

Use Docker when the sandbox does not have the project tooling installed locally, such as Go, Node, pnpm, PostgreSQL clients, or Playwright browsers. The goal is to keep verification moving without repeatedly asking the user to approve equivalent commands.

## Approval-Friendly Command Shape

The most reliable pattern is a direct command that starts with:

```bash
docker run --rm ...
```

Keep all options, environment variables, mounts, users, and working directories on the Docker command itself:

```bash
docker run --rm --network host --user 1000:1000 -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./...
```

Avoid shell constructs around Docker commands when approval reuse matters:

- Do not prefix with host-side `VAR=value`; use Docker `-e VAR=value`.
- Do not use `cd backend && ...`; use Docker `-w /repo/backend`.
- Do not pipe, redirect, or chain checks with `|`, `>`, `&&`, `;`, or command substitutions.
- Do not wrap the Docker run in `bash -lc` unless there is no direct equivalent.

The approval system evaluates command segments. Shell operators and host-side expansions can change the segment being evaluated and cause fresh approval requests even when the work is still just a Dockerized check.

## Common Flags

Use these defaults for repo checks:

```bash
docker run --rm --network host --user 1000:1000 -v /home/sciacco/devel/mrsmith:/repo -w /repo IMAGE COMMAND
```

- `--rm` keeps one-off check containers disposable.
- `--network host` avoids Docker DNS issues when downloading Go or Node dependencies and lets browser tests reach local dev servers.
- `--user 1000:1000` avoids root-owned build artifacts in the workspace.
- `-v /home/sciacco/devel/mrsmith:/repo` mounts the repository at a stable path.
- `-w /repo/...` replaces host-side `cd`.
- Write screenshots, temporary scripts, caches, and reports under `/tmp` when they do not belong in the repo.

When a tool writes caches and the container user cannot write its default home, place caches in `/tmp`:

```bash
docker run --rm --network host --user 1000:1000 -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./...
```

## Go

Format Training backend code:

```bash
docker run --rm --user 1000:1000 -v /home/sciacco/devel/mrsmith:/repo -w /repo golang:1.26.1 gofmt -w backend/internal/training
```

Run targeted backend tests:

```bash
docker run --rm --network host --user 1000:1000 -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./internal/training
```

Run the full backend suite:

```bash
docker run --rm --network host --user 1000:1000 -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./...
```

If dependency downloads fail with DNS or proxy errors, rerun with `--network host` before asking for broader approval.

## Node And pnpm

Run a package build through Corepack:

```bash
docker run --rm --network host --user 1000:1000 -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack -v /home/sciacco/devel/mrsmith:/repo -w /repo node:20-slim corepack pnpm --filter mrsmith-training build
```

For packages that require newer Node behavior, pick the matching image explicitly:

```bash
docker run --rm --network host --user 1000:1000 -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack -v /home/sciacco/devel/mrsmith:/repo -w /repo node:22-slim corepack pnpm --filter @mrsmith/auth-client test
```

Keep generated package artifacts out of the final change unless the task explicitly requires them.

## Playwright And Browser Checks

Before running Playwright, check whether the relevant dev server is already running and reuse it. Do not start a second Vite or preview server when an existing one is suitable.

Use direct Docker commands for browser checks, with screenshots under `/tmp`:

```bash
docker run --rm --network host --user 1000:1000 -e PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 -v /home/sciacco/devel/mrsmith:/repo -v /tmp/training-ui:/screenshots -w /repo node:20-slim node /screenshots/capture-training-ui.cjs
```

Notes:

- Use `--network host` so the container can reach `localhost` dev servers.
- Keep capture scripts and image output in `/tmp/<task-name>` unless they are intentional repo artifacts.
- If a browser bundle is already installed in an image or cache, set `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1`.
- When a dev server must be started, run it as a long-running session and stop it before final response.

## PostgreSQL And Migration Checks

For disposable database validation, run PostgreSQL in a named container and use direct `docker run --rm` clients connected through the container network:

```bash
docker run --rm --name mrsmith-training-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=anisetta postgres:16-alpine
```

Then apply or verify SQL from another one-off container:

```bash
docker run --rm --network container:mrsmith-training-pg -e PGPASSWORD=postgres -v /home/sciacco/devel/mrsmith:/repo postgres:16-alpine psql -h 127.0.0.1 -U postgres -d anisetta -v ON_ERROR_STOP=1 -f /repo/deploy/migrations/012_anisetta_training.sql
```

Keep throwaway verification SQL under `/tmp` unless it should become a permanent repo test or migration fixture.

## Troubleshooting

- Permission denied writing Go cache: add `-e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go`.
- Root-owned repo files: add `--user 1000:1000`.
- Dependency download DNS failures: add `--network host`.
- Approval requested again for an equivalent check: remove host-side shell operators, redirects, pipes, command substitutions, and leading environment assignments so the command starts with `docker run --rm`.
- Need several checks: run them as separate direct Docker commands rather than one chained shell command. This keeps output clearer and preserves approval matching.
