version: 1.5.0

## v1.5.0 - Shared reminders and shopping list editor

### What was done

Shopping list creation no longer uses the `Добавить в список` button. After opening the shopping-list draft, every user message is appended as a separate item while the current viewing, item toggle, pagination, and persistence behavior stays unchanged.

Reminder drafts now show `Напомнить: мне` by default. The user can press `Изменить получателя`, enter a recipient as `@username`, or press `Мне` to return to a personal reminder.

The bot stores Telegram usernames for users who have used EasyNote. If the selected recipient exists and already allowed the sender, the outgoing reminder is created as ready for execution. Otherwise, while incoming requests are enabled, the recipient receives `Разрешить пользователю @username добавить вам напоминание?` with `Разрешить`, `Запретить`, and `Разрешать всегда` actions.

Settings now include `Кто может мне напоминать` and `Запросы на напоминания: Включено/Выключено`. The access list is paginated like reminder lists, allows adding users by username, and allows removing previously allowed users. If incoming reminder requests are disabled, new reminders from other users are stored as declined.

The reminders menu now includes `Исходящие`, where users can see reminders they created for other users with statuses `Выполнено`, `Отменено`, `Напомнено`, `Ожидает исполнения`, `Отклонено`, `Ожидает подтверждения`, and `Не отправлено`.

### API changes

- added `bot_users` and `reminder_permissions` tables;
- added `incoming_reminder_requests_enabled` to `user_settings`;
- added sender and recipient metadata to `notes`;
- extended reminder statuses with `pending_confirmation` and `declined`;
- added PostgreSQL, service, and Telegram methods for shared reminder requests, permissions, outgoing reminders, and incoming request toggles.

### Affected files

- `CHANGELIST.md`
- `README.md`
- `README_RU.md`
- `migrations/00007_add_user_reminders.go`
- `src/database/postgresql/user_reminder_storage.go`
- `src/database/postgresql/reminders_storage.go`
- `src/database/postgresql/recurring_storage.go`
- `src/database/postgresql/user_settings_storage.go`
- `src/database/types.go`
- `src/service/reminders.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/sender.go`
- `src/telegram/ui`

## v1.4.0 - Holidays and shopping lists

### What was done

Telegram creation now supports three entry points: regular reminders, holidays, and shopping lists.

Holiday drafts render the congratulation template, collect holiday name, recipient name, date, and reminder time, and save holidays separately from regular notes and reminders. The scheduler sends due holidays through the planner, so holidays do not appear in active or past reminder lists. The main menu now has a `🎉 Праздники` section with two-items-per-page navigation, holiday details, edit, delete, and back actions.

Active reminder details now include an edit action that opens the reminder draft with the current title, description, date, and time.

Intrusive reminders now stop at 00:00 on the next local calendar day after the reminder time. When that cutoff is reached, the scheduler deletes the last intrusive reminder message and completes the reminder as skipped.

Shopping list drafts render numbered items under `Необходимо купить`, append new items as the user adds them, and persist lists with item state in PostgreSQL. The main menu now has a `🛒 Списки` section with two-items-per-page navigation. Opening a list shows each item as an inline button; pressing an item toggles its persisted checked state and shows the item with a check mark and strikethrough text.

The main menu reply keyboard was reordered to `➕ Создать`, then `⏰ Напоминания` / `🛒 Списки`, then `🎉 Праздники` / `⚙️ Настройки`.

Regular reminder draft buttons now place `Дата и время` below the `Название` / `Описание` row.

The visible reply-keyboard refresh message sent immediately before a reminder notification now says `Напоминаю:`.

### API changes

- added separate PostgreSQL storage methods for holidays and shopping lists;
- added `holidays`, `shopping_lists`, and `shopping_list_items` tables;
- added planner and scheduler paths for due holiday delivery.

### Affected files

- `CHANGELIST.md`
- `README.md`
- `README_RU.md`
- `migrations/00006_add_holidays_and_shopping_lists.go`
- `src/database/postgresql/holiday_storage.go`
- `src/database/postgresql/shopping_storage.go`
- `src/database/postgresql/planner_transaction_storage.go`
- `src/database/types.go`
- `src/planner/planner.go`
- `src/scheduler/ready_reminders.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/sender.go`
- `src/telegram/ui`

## v1.3.2 - Telegram UI recovery and PR deployment

### What was done

The Telegram main menu message was restored because Telegram cannot attach reply and inline keyboards to one message. The menu now shows completed, pending, and missed task counters in one formatted block. Missed tasks include canceled, skipped, and failed reminders.

The main menu stats block no longer uses a Markdown code block; it is rendered with Telegram HTML formatting and counts `canceled`, `skipped`, and `failed` reminders as missed tasks.

Reminder delivery now refreshes the reply main keyboard through a visible main-menu message and then sends the reminder itself with inline `Готово` and `Отменить` actions only. The sender keeps the last main-menu message id in memory per chat and deletes that previous menu before sending the next reminder.

Reminder creation returned to inline keyboards because the previous reply-keyboard editor made field editing and time selection harder to use. Filled draft fields still switch to `Изменить ...`, field buttons wait for user input again, and user input messages are deleted after handling.

Reminder creation text was returned to the sectioned layout with bold labels for `Название`, `Описание`, `Дата начала напоминаний`, and `Повторять`. Service prompts are rendered in italic.

The reminder draft no longer shows the extra template label. Empty title, description, and start date are rendered as empty field values under their labels.

The main draft inline keyboard is ordered as date/time, title/description, repeat, and save/cancel.

The `Название` and `Описание` draft buttons now switch the message into input-waiting mode by editing the existing message text, avoiding a delete/send blink. While the bot waits for input, the buttons show `Введите название` / `Введите описание`; after input they show `Изменить название` / `Изменить описание`.

Active reminders opened from the reply-keyboard list now show the inline reminder details keyboard again, including the `Удалить` action. Reminder list buttons no longer expose internal reminder IDs.

Delivered reminder actions now remove the notification message after `Готово` or `Отменить`.

Intrusive reminder settings now use an inline `Включено` / `Выключено` toggle plus a `Сохранить` action and accept manual interval input directly from chat, from 1 minute to 2 hours. Plain numbers are treated as minutes, and Go duration values such as `1m` and `1h30m` are supported.

Intrusive reminder settings now edit the existing settings message after manual input instead of deleting and sending a new message. Interval values are rendered as user-facing text, for example `каждые 30 минут` or `раз в 1 час 20 минут`.

The database interval constraint was relaxed to 1-120 minutes, so values below 30 minutes can be persisted and used by the scheduler.

Timezone reply-keyboard arrows are handled in the timezone context before reminder-list pagination, so they no longer open active reminders.

Settings now have an explicit `Сохранить` button that returns to the main menu. Timezone selection now contains 24 unique UTC hour offsets; `Калининград` is used for UTC+2, `Москва` is used for UTC+3, and the default timezone is Moscow.

Timezone buttons are now labeled uniformly as `UTC (+N)` / `UTC (-N)`, including the UTC+3 default option. Reminder drafts, active reminder lists, and reminder detail views load the user's timezone before calculating or formatting reminder times.

The reminder-list `Назад` button is no longer treated as a reminder title, so it returns to the reminders menu. Intrusive reminder settings now expose a `Назад` inline button that returns to settings.

The recurrence menu can now reset an already selected repeat mode with `Не повторять`.

Manual CI deployment from a pull request branch can now deploy an image tagged as `X.Y.Z.<short_sha>`.

### API changes

- added `ReminderStatsByChat` storage/repository method for main-menu counters;
- added the manual `workflow_dispatch.deploy_from_pr` CI input.

### Affected files

- `.github/workflows/ci.yml`
- `README.md`
- `README_RU.md`
- `migrations/00004_add_user_settings_and_intrusive_reminders.go`
- `migrations/00005_relax_intrusive_interval_and_default_timezone.go`
- `src/database/postgresql/reminders_storage.go`
- `src/database/types.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/sender.go`
- `src/telegram/ui`

## v1.3.1 - Reply keyboard reminder navigation

### What was done

Active and past reminder lists moved to reply keyboards with two reminders per page and previous/next navigation buttons.

Reminder list pagination is performed at the database query level through limit and offset instead of loading a larger set into UI code.

Reminder creation, settings, timezone, regularity, and intrusive reminder menus moved from inline keyboards to reply keyboards where Telegram supports that interaction.

The recurrence draft wording was updated: `Регулярность` became `Настроить регулярность`, `Повторяемость` became `Повторять`, and the empty value is shown as `нет`.

The recurrence menu replaced `Без повторений` with `По будням`, and intrusive reminder settings now use a single state button with `Включено` / `Выключено`.

### API changes

Reminder list repository methods now accept database-level pagination parameters.

### Affected files

- `src/database/postgresql/reminders_storage.go`
- `src/database/types.go`
- `src/service/reminders.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/ui`

## v1.3.0 - Reminder settings and persistent delivery

### What was done

Telegram reminder creation now accepts manual `HH:MM` input while the time picker is open. The bot updates the draft time and deletes the user's input message after handling it.

Quick time presets now finish time selection immediately and return the user to the reminder draft editor.

The recurring reminder menu now exposes only daily repeat, custom weekdays, no repeat, and a custom day-interval option. Removed options are no longer shown in the Telegram UI.

The main menu now includes settings and reminder history. Settings currently cover Russian language display, timezone selection by city, and intrusive reminder configuration with a 30 minute to 24 hour interval.

Intrusive reminders send `Готово` and `Отменить` actions with delivered reminders. When enabled, due reminders are repeated according to the user's interval, the previous reminder message is removed before the next one is sent, and reminders older than 24 hours are completed as skipped.

Reminder history shows completed, canceled, skipped, sent, and failed reminder statuses separately from active reminders.

### API changes

- added `user_settings`;
- extended `reminders` with delivery state columns for intrusive reminders;
- added `completed` and `skipped` reminder statuses;
- scheduler sender now returns a Telegram message id for persisted delivery state.

### Affected files

- `migrations/00004_add_user_settings_and_intrusive_reminders.go`
- `src/database/postgresql/reminders_storage.go`
- `src/database/postgresql/user_settings_storage.go`
- `src/database/types.go`
- `src/scheduler/ready_reminders.go`
- `src/service/reminders.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/sender.go`
- `src/telegram/ui`

## v1.2.0 - Scheduler runtime runner

### What was done

The reminder runtime worker was moved from `service` to `scheduler` as `scheduler.Runner`.

The old heap/cache scheduler model was removed, including queue/`Next(ctx)` abstractions, cache-window loading, old scheduler tests, generated mocks for that model, and the obsolete `src/database/cache` package.

Scheduler now owns send retries in the runtime layer. `send_retry_count` configures the total send attempts for one ready reminder, `send_retry_interval` configures the constant or initial backoff delay, `send_retry_max_interval` caps exponential backoff, and `send_retry_backoff` selects the retry strategy.

The runner uses the existing `Planner` to complete sent or failed occurrences. Recurring reminders continue to use the existing planner completion flow; scheduler does not calculate recurrence and PostgreSQL remains storage.

Telegram reminder draft buttons now reflect field state. Recurring drafts show `Дата и время начала` before the start value is set and `Изменить дату и время начала` after it is set. Title, description, and date buttons also switch to `Изменить ...` when filled.

The Telegram bot deletes user messages produced by the main reply-keyboard commands `Создать напоминание` and `Список напоминаний` after handling them. Deletion failures are logged without breaking command handling.

### API changes

Scheduler configuration changed:

- removed `cache_window`;
- removed `max_attempts`;
- added `send_retry_count`;
- added `send_retry_interval`.
- added `send_retry_max_interval`;
- added `send_retry_backoff`.

### Affected files

- `config/config.yaml`
- `main.go`
- `README.md`
- `README_RU.md`
- `src/config/config.go`
- `src/database/postgresql/reminders_storage.go`
- `src/database/cache/heap.go`
- `src/database/cache/reminder_cache.go`
- `src/database/cache/reminder_cache_test.go`
- `src/scheduler/clock.go`
- `src/scheduler/runner.go`
- `src/scheduler/runner_test.go`
- `src/scheduler/mocks/scheduler_mock.go`
- `src/scheduler/scheduler.go`
- `src/scheduler/scheduler_test.go`
- `src/scheduler/storage.go`
- `src/service/reminder_worker.go`
- `src/service/reminder_worker_test.go`
- `src/telegram/reminder_bot.go`
- `src/telegram/reminder_bot_test.go`
- `src/telegram/ui.go`

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
