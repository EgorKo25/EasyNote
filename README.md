# EasyNote

Telegram reminder bot for routine tasks.

## Version

The source of truth for the application version is `CHANGELIST.md`.

Current version: `1.3.2`.

## Tests

Focused verification commands:

```bash
go test ./src/scheduler ./src/telegram ./src/config -cover
golangci-lint run --config ./.golangci.yml
```

Deployment-focused checks:

```bash
go test ./src/config ./src/telegram -count=1
golangci-lint run --config ./.golangci.yml ./src/config
docker build --check -f deploy/docker/EasyNote.Dockerfile .
```

### Package `src/scheduler`

The scheduler package is the runtime layer for sending ready reminders. `scheduler.Runner` replaced the old service reminder worker and the old heap/cache scheduler model.

Runtime behavior:

- The runner waits for `scheduler.poll_interval`.
- It asks storage for `new` reminder occurrences whose `remind_at` is due.
- It sends each reminder through the configured sender.
- Sender failures are retried in scheduler with `scheduler.send_retry_count` and `scheduler.send_retry_interval`.
- Retry intervals are produced by the scheduler backoff adapter; `send_retry_backoff` supports `constant` and `exponential`.
- Successful delivery is completed through the existing `Planner` as `sent`.
- Exhausted send retries are completed through the existing `Planner` as `failed`.
- The runner logs send failures and continues with the next reminder when failed completion succeeds.
- Recurring reminders are still handled by `Planner`; scheduler does not calculate recurrence.

The old heap/cache scheduler, `Next(ctx)` queue, `ActualReminders` window loading, and `src/database/cache` package were removed.

The package tests use generated gomock interfaces and controlled time from `github.com/benbjohnson/clock`; retry intervals are tested without real sleep.

| Scenario | What is verified | Test type | Status |
|---|---|---|---|
| Ready reminder batch | Ready reminders are loaded, sent, and completed as sent | table-driven, mock storage/sender/planner | PASS |
| Empty batch | No sender or planner calls are made | table-driven, mock storage | PASS |
| Retry success | Sender failure waits on fake clock and then completes as sent | table-driven, mock clock | PASS |
| Retry exhausted | Reminder is completed as failed and later reminders continue | table-driven, mock clock | PASS |
| Storage failure | Storage error is returned | table-driven, mock storage | PASS |
| Planner failure | Completion error is returned | table-driven, mock planner | PASS |
| Context cancellation | Run exits on canceled context | table-driven, mock clock | PASS |

### Packages `src/database` and `src/config`

These packages are included in the focused command because scheduler and Telegram depend on their contracts.

| Package | Current focused status |
|---|---|
| `src/database` | no test files |
| `src/config` | no package-specific tests in this focused run |

## Architecture

EasyNote follows a thin clean-architecture style around the reminder flow:

- `service` owns Telegram-facing reminder validation and user workflows.
- `planner` owns reminder lifecycle orchestration, recurring-date calculation, and completion idempotency.
- `database/postgresql` is the storage/repository layer for notes, concrete reminder firings, recurring rules, custom dates, and transactions.
- `scheduler` owns the runtime loop that sends ready reminders.
- `telegram` owns the user flow and delegates reminder persistence to the service layer.
- `main` wires PostgreSQL, the Telegram sender, the scheduler runner, and the Telegram bot.

Data flow:

1. `service` receives a user action from the Telegram bot.
2. PostgreSQL stores the note and one concrete row in `reminders`.
3. `scheduler.Runner` polls `reminders` rows with status `new` and due `remind_at`.
4. After delivery, scheduler calls `Planner.CompleteOccurrence`.
5. Planner marks the current row as `sent` or `failed` and, for active recurring rules, creates the next concrete reminder in the same transaction.

The design keeps recurrence calculation in the planner layer and uses PostgreSQL for storage and transactionality.

### Recurring Reminders

One row in `reminders` is one concrete firing at one `remind_at`. Recurring reminders are represented by a separate `reminder_rules` row and a nullable `reminders.rule_id`.

Supported recurrence types:

- Interval rules: every day, every 2 days, and every week.
- Weekday rules: workdays or a custom selected weekday mask.
- Custom date rules: several selected calendar dates stored in `reminder_custom_dates`.

The scheduler never recalculates recurrence itself. It sends due reminders and calls planner completion logic. Completion is transactional: current reminder status is updated, the rule is checked, and the next reminder is inserted or the rule is completed. PostgreSQL also has a partial unique index that allows only one active `new` reminder for a non-null `rule_id`.

### Scheduler Configuration

```yaml
scheduler:
  poll_interval: 1m
  batch_limit: 50
  send_retry_count: 3
  send_retry_interval: 5s
  send_retry_max_interval: 30s
  send_retry_backoff: constant
```

`send_retry_count` is the total number of send attempts for one ready reminder. `send_retry_interval` is the constant delay or exponential initial delay. `send_retry_max_interval` caps exponential backoff. `send_retry_backoff` selects `constant` or `exponential`.

### Telegram UI

The main menu is a reply-keyboard message with a compact formatted stats block for completed, pending, and missed tasks. Missed tasks include canceled, skipped, and failed reminders.

Reminder creation uses inline keyboards again. Draft buttons reflect filled fields and expose direct edit actions for title, description, date, repeat settings, saving, and canceling. While the bot waits for title or description input, those buttons change to `Введите название` or `Введите описание`; after input they change to `Изменить ...`.

The draft screen is marked as a template, centers the title, separates description, repeat state, and start time with blank lines, keeps the start time on one line, and shows service prompts in italic. The prompt only asks for missing required fields: title, time, or both.

Reminder time is entered as text while the time screen is open, in `HH:MM` format.

Active reminders opened from the reply-keyboard list show inline details actions, including `Удалить`.

Intrusive reminder settings use an inline `Включено` / `Выключено` toggle and edit the existing settings message after manual interval input. Intervals can be entered as plain minutes or Go duration values such as `1m` and `1h30m`, and are displayed as user-facing text such as `каждые 30 минут` or `раз в 1 час 20 минут`.

Settings have an explicit `Сохранить` button that returns to the main menu. Timezone selection includes Russian and worldwide cities, with `Калининград` placed before `Москва`.

Reminder notifications keep the main reply keyboard available by briefly sending and deleting a main-menu message before the inline reminder notification. The notification itself uses only inline `Готово` and `Отменить` actions and is deleted after either action.

After reply-keyboard commands and user text inputs are handled, the bot deletes those user messages from the dialog and logs deletion failures without interrupting the command.

## Deployment

Docker deployment files live in `deploy/docker`.

Local Compose startup from the current working tree:

```bash
cd deploy/docker
cp .env .env.local
# Fill TELEGRAM_BOT_TOKEN in .env.local
docker compose -f docker-compose.local.yaml up -d --build
docker compose -f docker-compose.local.yaml logs -f app
```

Local or server-side Compose startup:

```bash
cd deploy/docker
docker compose up -d postgres
docker compose up -d app
```

Required application environment:

| Variable | Purpose |
|---|---|
| `APP_IMAGE` | Application image used by Compose, for example `ghcr.io/owner/repository:beta` |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token; startup fails when it is missing |
| `POSTGRES_DB` | PostgreSQL database name |
| `POSTGRES_USER` | PostgreSQL user |
| `POSTGRES_PASSWORD` | PostgreSQL password |
| `POSTGRES_HOST_PORT` | Optional host port for PostgreSQL; local Compose defaults to `15432` |
| `HTTP_PROXY` | Optional proxy URL for Telegram API requests |
| `HTTPS_PROXY` | Optional proxy URL for Telegram API requests |
| `NO_PROXY` | Optional comma-separated list of hosts that bypass the proxy |

The application config is loaded in this order:

1. Struct defaults.
2. `config/config.yaml`.
3. Environment overrides parsed by `github.com/caarlos0/env/v11`.
4. Validator checks.

Supported deployment overrides:

| Variable | Config field |
|---|---|
| `APP_ENVIRONMENT` | `app.environment` |
| `DATABASE_HOST` | `database.host` |
| `DATABASE_PORT` | `database.port` |
| `DATABASE_USER` | `database.user` |
| `DATABASE_PASSWORD` | `database.password` |
| `DATABASE_NAME` | `database.name` |
| `DATABASE_SSL_MODE` | `database.ssl_mode` |

### Beta CD

The GitHub Actions workflow has a manual `workflow_dispatch` beta deployment path.

Repository/environment variables and secrets required by the beta job:

| Name | Type | Purpose |
|---|---|---|
| `BETA_DEPLOY_PATH` | variable | Directory on the server where `docker-compose.yaml` is uploaded |
| `BETA_SSH_HOST` | secret | Server host |
| `BETA_SSH_USERNAME` | secret | SSH user |
| `BETA_SSH_KEY` | secret | SSH private key |
| `BETA_SSH_PORT` | secret, optional | SSH port, defaults to `22` |
| `GHCR_TOKEN` | secret, optional | GHCR token; falls back to `GITHUB_TOKEN` |

The deploy job pushes the `beta` image tag, uploads the Compose file, pulls the new application image, and restarts only the application service:

```bash
docker compose pull app
docker compose up -d --no-deps app
```

PostgreSQL is not restarted by the beta CD job.

For a manual deployment from a pull request branch, run the same workflow manually on the PR branch and enable `deploy_from_pr`. The image tag is built as `X.Y.Z.<short_sha>`, for example `1.3.2.a1b2c3d`, and that exact tag is deployed.

## Changelist

<div style="max-height: 420px; overflow-y: auto; border: 1px solid #d0d7de; border-radius: 6px; padding: 16px;">
<p><strong>version: 1.3.2</strong></p>

<h2>v1.3.2 - Telegram UI recovery and PR deployment</h2>
<ul>
  <li>The main menu message was restored with formatted completed, pending, and missed task counters; missed includes canceled, skipped, and failed reminders.</li>
  <li>Reminder notifications refresh the reply keyboard through a temporary main-menu message and keep inline actions on the notification itself.</li>
  <li>Reminder creation returned to inline keyboards; draft prompts ask only for missing title/time fields, title/description buttons show input-waiting state, and reminder time is typed as HH:MM.</li>
  <li>Intrusive reminder settings use an inline toggle, edit the existing message after manual input, and accept minute or duration values such as 1m and 1h30m.</li>
  <li>Manual PR deployment can publish and deploy an image tagged as X.Y.Z.&lt;short_sha&gt;.</li>
</ul>

<h2>v1.3.1 - reply keyboard reminder navigation</h2>
<ul>
  <li>Active and past reminder lists use reply keyboards with two reminders per page and previous/next navigation.</li>
  <li>Reminder list pagination is performed in database queries with limit and offset.</li>
  <li>Creation, settings, timezone, regularity, and intrusive reminder menus moved to reply keyboards.</li>
  <li>The recurrence wording was updated to "Настроить регулярность", "Повторять", and "нет".</li>
  <li>The recurrence menu added "По будням", and intrusive reminders use one state button: "Включено" or "Выключено".</li>
</ul>

<h2>v1.2.0 - scheduler runtime runner</h2>
<ul>
  <li>The reminder runtime worker moved from service to scheduler.</li>
  <li>The old heap/cache scheduler and database cache package were removed.</li>
  <li>Scheduler send retries are configured with send_retry_count, send_retry_interval, send_retry_max_interval, and send_retry_backoff.</li>
  <li>Reminder draft buttons and main-menu message cleanup were improved.</li>
</ul>

<h2>v1.1.0 - recurring reminders</h2>
<ul>
  <li>Recurring reminders are backed by rule rows and concrete reminder firings.</li>
  <li>Supported modes include intervals, workdays, custom weekdays, and selected dates.</li>
  <li>Reminder completion creates the next firing transactionally in PostgreSQL.</li>
  <li>The recurrence lifecycle is now orchestrated by a planner layer above PostgreSQL storage.</li>
</ul>

<h2>v1.0.0 - beta deployment ready</h2>
<ul>
  <li>The beta deployment path is ready for manual GitHub Actions runs.</li>
  <li>The deploy job updates only the application container and keeps PostgreSQL running.</li>
  <li>The server path and SSH connection are supplied by GitHub variables and secrets.</li>
</ul>

<h2>v0.9.0 - manual CD through GitHub Actions</h2>
<ul>
  <li>`workflow_dispatch` was added for deliberate beta deployments.</li>
  <li>The Docker image receives a `beta` tag during manual runs.</li>
  <li>The deploy step uploads Compose, pulls the app image, and runs `docker compose up -d --no-deps app`.</li>
</ul>

<h2>v0.8.0 - Docker deployment layout</h2>
<ul>
  <li>Docker files were grouped under `deploy/docker`.</li>
  <li>The deployment Dockerfile now builds the root application package and copies the existing config directory.</li>
  <li>The Compose file defines app, PostgreSQL, persistent database data, and logs.</li>
</ul>

<h2>v0.7.0 - environment-backed runtime configuration</h2>
<ul>
  <li>Environment overrides are parsed with `github.com/caarlos0/env/v11` after defaults and YAML.</li>
  <li>Database and application environment settings can be supplied by deployment env vars.</li>
</ul>

<h2>v0.6.0 - Telegram token supplied from configuration</h2>
<ul>
  <li>The Telegram token is read from the environment variable named by `telegram.token_env`.</li>
  <li>The default token variable is `TELEGRAM_BOT_TOKEN`, and startup fails if it is missing.</li>
</ul>

<h2>v0.5.0 - background reminder runtime</h2>
<ul>
  <li>The runtime now uses background processes for reminder sending and Telegram long polling.</li>
  <li>The reminder worker polls the database for due reminders and updates delivery status.</li>
</ul>

<h2>v0.4.0 - scheduler uses database as source of truth</h2>
<ul>
  <li>Scheduler persists reminders through the Database interface before caching.</li>
  <li>Cache refresh now replaces the active reminder window.</li>
  <li>Scheduler tests cover real cache behavior, mock database behavior, mock cache delegation, and load scenarios.</li>
</ul>

<h2>v0.3.0 - added pull request CI and versioned build artifacts</h2>
<ul>
  <li>GitHub Actions CI was added for pull requests.</li>
  <li>Build artifacts and Docker image tags use the changelist version.</li>
</ul>

<h2>v0.2.0 - added reminder cache and repository quality checks</h2>
<ul>
  <li>Reminder cache, heap scheduling primitives, migrations, and strict golangci-lint configuration were added.</li>
</ul>

<h2>v0.1.0 - created base application structure</h2>
<ul>
  <li>Initial Go module, configuration, logger, and reminder cache were added.</li>
</ul>
</div>
