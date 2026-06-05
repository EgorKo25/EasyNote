version: 1.1.0

## v1.1.0 - Recurring reminders

### What was done

Recurring reminders are now supported through separate rule storage instead of rewriting old reminder rows.

The `reminders` table remains the table of concrete firings: one row is one reminder at one `remind_at`. Recurring rules are stored in `reminder_rules`, custom selected dates are stored in `reminder_custom_dates`, and recurring reminder rows reference rules through nullable `reminders.rule_id`.

Supported recurrence modes:

- every day;
- every 2 days;
- every week;
- workdays;
- custom selected weekdays;
- custom selected calendar dates.

Reminder completion now runs through the `planner` layer. It marks the current reminder as `sent` or `failed`, loads the active rule, calculates the next firing in Go, and inserts the next `reminders` row or completes the rule in one transaction.

PostgreSQL remains the storage/repository layer: it reads and writes notes, reminders, rules, custom dates, and transactional locks, but it no longer owns recurring reminder formulas or lifecycle orchestration.

Telegram reminder creation now includes recurring reminder controls, custom weekday selection, multi-date custom selection, and cleanup of user command/input messages after they are handled.

### API changes

No public HTTP API was introduced.

### Affected files

- `migrations/00003_add_recurring_reminder_rules.go`
- `src/database/postgresql/postgresql.go`
- `src/database/postgresql/reminders_storage.go`
- `src/database/postgresql/recurring_storage.go`
- `src/database/postgresql/transaction_storage.go`
- `src/database/types.go`
- `src/planner/planner.go`
- `src/planner/occurrences.go`
- `src/service/reminders.go`
- `src/service/reminder_worker.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/ui.go`

## v1.0.3 - Telegram reminder UI and delivery fixes

### What was done

Reminder creation no longer offers manual time input. The date step starts with `Сегодня`, `Завтра`, and `Выбрать дату`; the calendar is opened only on demand, past calendar days are disabled, and saving a reminder in the past is rejected with a clear message.

The reminder UI now has explicit navigation from reminder details back to the active reminder list, shows active reminders as title buttons, deletes user messages that contain reminder title or description, and offers main menu, active reminders, and creating another reminder after a reminder is created.

Due reminder delivery was fixed by correcting the SQL query that loads reminders ready to send.

### API changes

No public HTTP API was introduced.

## v1.0.2 - Reminder status simplification

### What was done

Reminder statuses were simplified to `new`, `sent`, `failed`, and `canceled`. The temporary `processing` state was removed so active and due reminders are selected from `new` only.

### API changes

No public HTTP API was introduced.

## v1.0.1 - Telegram UI bugfixes

### What was done

Telegram reminder creation no longer requires manual date and time input. The bot now uses an inline date and time picker, edits the active UI message during callbacks, and keeps reminder lists in one message with delete buttons.

The reminder UI now keeps user messages in the chat, has a main-menu action from the active reminder list, and offers a compact time editor with quick presets plus optional manual `HH:MM` input.

Due reminder delivery now retries reminders that were left in `processing` after an interrupted worker run, and terminally failed reminders are no longer shown as active.

Docker Compose and the beta deploy workflow now pass optional proxy variables from GitHub Secrets into the application container so Telegram API requests can use the deployment proxy.

### API changes

No public HTTP API was introduced.

## v1.0.0 - beta deployment ready

### What was done

The branch is now ready for a manually triggered beta deployment.

The Docker Compose deployment uses the GHCR `beta` image tag and keeps PostgreSQL as a persistent service that is not restarted during application-only deploys.

The manual GitHub Actions CD job validates deployment variables, uploads the Compose file to the configured server path, pulls the latest application image, and restarts only the application container.

### API changes

No public HTTP API was introduced.

The operational deployment contract is now stable for beta:

- `BETA_DEPLOY_PATH` defines where Compose is placed on the server.
- `BETA_SSH_HOST`, `BETA_SSH_USERNAME`, and `BETA_SSH_KEY` define server access.
- `TELEGRAM_BOT_TOKEN` must exist in the server `.env`, otherwise the application does not start.

### Affected files

- `.github/workflows/ci.yml`
- `deploy/docker/.env.example`
- `deploy/docker/docker-compose.yaml`

## v0.9.0 - manual CD through GitHub Actions

### What was done

The CI workflow was extended with `workflow_dispatch` so beta deployment is a deliberate manual action.

The Docker build step now uses the Dockerfile from `deploy/docker` and publishes a `beta` image tag for manual deployment runs.

The deploy job copies the Compose file to the server and runs `docker compose pull app` plus `docker compose up -d --no-deps app`.

### API changes

No runtime API changes were introduced.

The CI/CD contract changed: beta deployment is available only from a manual workflow run.

### Affected files

- `.github/workflows/ci.yml`

## v0.8.0 - Docker deployment layout

### What was done

Docker deployment files were moved into `deploy/docker`.

The deployment Dockerfile now builds the real root application package and copies the existing `config` directory into the runtime image.

The old root Dockerfile was removed so Docker-related files are grouped in one deployment directory.

The Compose file defines the application and PostgreSQL services, persistent database and log volumes, and a required Telegram token variable.

### API changes

No application API changes were introduced.

The Docker build command now uses `deploy/docker/EasyNote.Dockerfile`.

### Affected files

- `Dockerfile`
- `deploy/docker/.env.example`
- `deploy/docker/EasyNote.Dockerfile`
- `deploy/docker/docker-compose.yaml`

## v0.7.0 - environment-backed runtime configuration

### What was done

Configuration loading now applies environment variable overrides after defaults and YAML parsing.

The environment parser uses `github.com/caarlos0/env/v11` instead of hand-written `os.Getenv` mapping.

Database connection settings and application environment can be supplied by deployment environment variables.

### API changes

The following environment variables can override YAML configuration:

- `APP_ENVIRONMENT`
- `DATABASE_HOST`
- `DATABASE_PORT`
- `DATABASE_USER`
- `DATABASE_PASSWORD`
- `DATABASE_NAME`
- `DATABASE_SSL_MODE`

### Affected files

- `go.mod`
- `go.sum`
- `src/config/config.go`

## v0.6.0 - Telegram token supplied from configuration

### What was done

Telegram sender initialization reads the token from the environment variable named by `telegram.token_env`.

The default token environment variable is `TELEGRAM_BOT_TOKEN`.

Application startup fails during Telegram sender initialization when the token is missing.

### API changes

`telegram.token_env` is the configuration key for the Telegram token environment variable name.

### Affected files

- `config/config.yaml`
- `src/config/config.go`
- `src/telegram/sender.go`

## v0.5.0 - background reminder runtime

### What was done

The application runtime was wired around background processes for reminder sending and Telegram long polling.

The reminder worker polls the database for due reminders, sends them through Telegram, and updates reminder status.

The background runner owns long-lived process lifecycle and shared cancellation.

### API changes

The active reminder sending path is database-driven and does not require restarting or rebuilding an in-memory scheduler queue.

### Affected files

- `main.go`
- `src/background/background.go`
- `src/service/reminder_worker.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/sender.go`

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
