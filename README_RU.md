# EasyNote

Telegram-бот для напоминаний о рутинных задачах.

## Версия

Источник версии приложения - `CHANGELIST.md`.

Текущая версия: `0.4.0`.

## Тесты

Команды для focused-проверки:

```bash
go test ./src/scheduler ./src/database/cache ./src/database ./src/config -cover
golangci-lint run ./src/scheduler ./src/database/cache ./src/database ./src/config
```

### Пакет `src/scheduler`

Тесты планировщика проверяют контракт слоя service. Планировщик находится над слоем базы данных и cache и не зависит от конкретной реализации:

- `scheduler.Database` является источником истины.
- `scheduler.Cache` используется как оптимизация для активного окна напоминаний.
- `github.com/benbjohnson/clock` используется, чтобы управлять временем в тестах.

Поведение:

- новое напоминание всегда сохраняется через `Database.SaveScheduledReminder`;
- если напоминание попадает в текущее окно cache, оно дополнительно кладется в cache;
- если напоминание выходит за пределы окна cache, оно остается только в базе;
- `Refresh` загружает окно `[now, now + cacheWindow]` из базы и полностью заменяет содержимое cache.

Тесты проверяют этот контракт через реальный cache, сгенерированные gomock-интерфейсы базы/cache и управляемое время через `github.com/benbjohnson/clock`.

| Сценарий | Что проверяется | Тип теста | Статус |
|---|---|---|---|
| Schedule внутри cache window | Напоминание сохраняется в БД и попадает в реальный cache | table-driven, real cache, mock DB | PASS |
| Schedule вне cache window | Напоминание сохраняется в БД, но не попадает в cache | table-driven, real cache, mock DB | PASS |
| Schedule на границах времени | Текущий момент и точная граница окна принимаются | table-driven, real cache, mock DB | PASS |
| Ошибка сохранения | Ошибка БД оборачивается, cache не меняется | table-driven, mock DB | PASS |
| Due reminders | Готовые напоминания извлекаются по управляемому clock | table-driven, real cache | PASS |
| Refresh окна БД | Из БД запрашивается `[now, now + cacheWindow]` | table-driven, mock DB, real cache | PASS |
| Refresh заменяет cache | Старые элементы cache удаляются после refresh | table-driven, mock DB, real cache | PASS |
| Делегирование в cache | Scheduler вызывает due/cancel/delay/window/len у cache | table-driven, mock cache | PASS |
| 1 000 напоминаний | Scheduler сохраняет и извлекает 1 000 напоминаний | table-driven, real cache, mock DB | PASS |
| 10 000 напоминаний | Scheduler сохраняет и извлекает 10 000 напоминаний | table-driven, real cache, mock DB | PASS |
| 800 параллельных schedule | Scheduler принимает 800 параллельных вызовов планирования | table-driven, real cache, mock DB | PASS |
| 900 параллельных schedule | Scheduler принимает 900 параллельных вызовов планирования | table-driven, real cache, mock DB | PASS |

Покрытие: `100.0%`.

Нагрузочный benchmark планировщика:

```bash
go test ./src/scheduler -run '^$' -bench BenchmarkSchedulerScheduleAndDrainWithRealCache -benchtime=3x -benchmem
```

| Benchmark | Последний локальный результат |
|---|---:|
| `BenchmarkSchedulerScheduleAndDrainWithRealCache` | `~19.2 ms/op`, `~6.9 MB/op`, `~50160 allocs/op` |

### Пакет `src/database/cache`

Тесты cache проверяют heap-хранилище напоминаний, которое планировщик использует как оптимизационный слой.

| Сценарий | Что проверяется | Тип теста | Статус |
|---|---|---|---|
| Конвертация scheduled reminder | Поля note и reminder копируются в DTO планировщика | table-driven, real types | PASS |
| Heap operations | Push, pop, swap, ordering и heap indexes работают корректно | table-driven, real heap | PASS |
| Add within cache window | Только напоминания внутри текущего окна попадают в cache | table-driven, real cache | PASS |
| Upsert | Существующие напоминания обновляются без дублей | table-driven, real cache | PASS |
| Pop due | Готовые напоминания извлекаются, будущие остаются в cache | table-driven, real cache | PASS |
| Remove | Существующие напоминания удаляются, отсутствующие игнорируются | table-driven, real cache | PASS |
| Empty state | Операции над пустым cache остаются стабильными | table-driven, real cache | PASS |

Покрытие: `90.3%`.

### Пакеты `src/database` и `src/config`

Эти пакеты включены в focused-команду, потому что scheduler и cache зависят от их контрактов.

| Пакет | Текущий focused-статус |
|---|---|
| `src/database` | нет тестовых файлов |
| `src/config` | нет отдельного package-specific теста в этом focused-прогоне |

## Архитектура

EasyNote использует тонкий clean-architecture-подход вокруг потока напоминаний:

- `service` владеет use case и обращается к `scheduler`.
- `scheduler` координирует сохранение напоминаний, обновление cache и извлечение due-элементов.
- `database` является источником истины для напоминаний и данных заметок.
- `database/cache` держит in-memory heap как windowed-оптимизацию.
- `main` связывает конкретные реализации со слоями scheduler и service.

Поток данных:

1. `service` получает пользовательское действие от Telegram-бота.
2. `scheduler` сохраняет напоминание через интерфейс базы данных.
3. Если напоминание попадает в активное окно cache, `scheduler` дополнительно кладет его в cache.
4. `Refresh` загружает текущее окно из базы и полностью заменяет содержимое cache.
5. `Due` извлекает готовые напоминания из cache для дальнейшей отправки сервисным слоем.

Такой дизайн держит бизнес-правила выше деталей хранения и позволяет cache оставаться заменяемой оптимизацией.

### Работа cache

![Поток cache и heap в EasyNote](pictures/cache-heap-flow-ru.png)

Cache в этом приложении не является источником истины. Это узкий и быстрый слой, который держит только те напоминания, которые должны сработать в ближайшее время.

Что происходит по шагам:

1. `service` передает в `scheduler` запрос на сохранение напоминания.
2. `scheduler` всегда сначала пишет напоминание в базу данных.
3. Затем `scheduler` проверяет активное окно cache.
4. Если напоминание попадает в окно, оно добавляется в heap-backed cache.
5. Если напоминание вне окна, cache его пропускает, а база сохраняет его до следующего refresh.
6. Когда clock доходит до времени срабатывания, `Due()` снимает вершину heap и возвращает готовые напоминания.
7. Когда `service` или фоновый цикл вызывает `Refresh()`, `scheduler` загружает из БД текущий диапазон `[now, now + cacheWindow]` и полностью заменяет содержимое cache.

Как работает heap:

- `Add` вставляет напоминание и сохраняет самое раннее значение наверху.
- `Upsert` обновляет существующее напоминание без дублей.
- `PopDue` удаляет все напоминания, которые уже должны были сработать.
- `Remove` удаляет одно напоминание по id.
- `Replace` выбрасывает устаревшее содержимое cache и собирает активное окно заново из базы.

Поэтому heap может оставаться маленьким, а база данных хранит всю историю напоминаний.

## Changelist

<div style="max-height: 420px; overflow-y: auto; border: 1px solid #d0d7de; border-radius: 6px; padding: 16px;">
<p><strong>version: 0.4.0</strong></p>

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
