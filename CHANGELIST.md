version: 0.4.0

## v0.4.0 - scheduler uses database as source of truth

### What was done

The scheduler branch introduced a service-facing scheduler facade above the database cache layer.

The scheduler now persists every scheduled reminder through the `Database` interface before trying to cache it.

The cache can now replace its whole reminder window during refresh, so refreshed database windows remove stale cache entries.

Scheduler tests were rewritten as table-driven tests with `github.com/benbjohnson/clock`, real cache coverage, and generated `go.uber.org/mock` mocks.

Scheduler load coverage was added for 1,000 and 10,000 reminder scenarios, plus a benchmark for schedule-and-drain behavior.

Concurrent scheduler coverage was added for 800 and 900 parallel scheduling calls.

### API changes

`scheduler.Database` now exposes `SaveScheduledReminder(ctx, reminder)` and `ScheduledReminders(ctx, from, to)`.

`scheduler.Cache` now exposes `Replace(reminders)`.

`Scheduler.Schedule` and `Scheduler.ScheduleReminder` now accept `context.Context` and return `(bool, error)`.

`Scheduler.Refresh` now loads the `[now, now + cacheWindow]` database window and replaces the cache contents.

### Affected files

- `CHANGELIST.md`
- `README.md`
- `README_RU.md`
- `.gitignore`
- `go.mod`
- `go.sum`
- `src/database/cache/heap.go`
- `src/database/cache/reminder_cache.go`
- `src/database/cache/reminder_cache_test.go`
- `src/database/types.go`
- `src/scheduler/clock.go`
- `src/scheduler/dependencies.go`
- `src/scheduler/mocks/scheduler_mock.go`
- `src/scheduler/scheduler.go`
- `src/scheduler/scheduler_test.go`

## v0.3.0 - added pull request CI and versioned build artifacts

### What was done

GitHub Actions CI was added for pull requests.

The pipeline builds the Go application, uploads a versioned binary artifact, runs golangci-lint from the repository config, runs unit tests, and builds/pushes the Docker image to GHCR.

The CI artifact and Docker image tags use the version from `CHANGELIST.md`.

### API changes

No runtime API changes were introduced.

The delivery contract changed: CI artifacts and Docker image tags are versioned from the changelist.

### Affected files

- `.dockerignore`
- `.github/workflows/ci.yml`
- `CHANGELIST.md`
- `Dockerfile`
- `README.md`

## v0.2.0 - added reminder cache and repository quality checks

### What was done

The reminder cache layer was extended with heap-based scheduling primitives and unit test coverage.

The repository lint configuration was added and existing code was adjusted to match the configured checks.

The database migration and shared database types for notes and reminders were added.

### API changes

No public HTTP or external API changes were introduced.

The internal database and cache APIs were extended for reminder storage and retrieval.

### Affected files

- `.gitignore`
- `.golangci.yml`
- `config/config.yaml`
- `go.mod`
- `go.sum`
- `main.go`
- `migrations/00001_init_notes_and_reminders.go`
- `src/config/config.go`
- `src/database/cache/heap.go`
- `src/database/cache/reminder_cache.go`
- `src/database/cache/reminder_cache_test.go`
- `src/database/types.go`
- `src/logger/logger.go`

## v0.1.0 - created base application structure

### What was done

The initial Go module and application entrypoint were created.

The base configuration loader was added with YAML parsing, defaults, and validation.

The logger abstraction and Zap-based implementation were added.

The first reminder cache implementation was added.

### API changes

No public HTTP or external API was introduced.

The initial internal configuration, logger, and reminder cache APIs were introduced.

### Affected files

- `.gitignore`
- `config/config.yaml`
- `go.mod`
- `go.sum`
- `main.go`
- `src/config/config.go`
- `src/database/cache/reminder_cache.go`
- `src/logger/api.go`
- `src/logger/logger.go`
