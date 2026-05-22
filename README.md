# EasyNote

Telegram бот для того, чтобы напоминать вам о рутинных делах.

## Changelist

Источник текущей версии приложения: `CHANGELIST.md`.

<div style="max-height: 420px; overflow-y: auto; border: 1px solid #d0d7de; border-radius: 6px; padding: 16px;">
<p><strong>version: 0.2.0</strong></p>

<h2>v0.2.0 - добавлен кеш напоминаний и строгие проверки репозитория</h2>

<h3>Что было сделано</h3>
<ul>
  <li>Был расширен слой кеша напоминаний: добавлены heap-примитивы для планирования и unit-тесты.</li>
  <li>Был добавлен конфиг golangci-lint, а существующий код был приведен к настроенным проверкам.</li>
  <li>Были добавлены миграция базы данных и общие типы базы данных для заметок и напоминаний.</li>
</ul>

<h3>Изменения API</h3>
<ul>
  <li>Публичный HTTP API и внешний API приложения не изменялись.</li>
  <li>Внутренние API базы данных и кеша были расширены для хранения и получения напоминаний.</li>
</ul>

<h3>Затронутые файлы</h3>
<ul>
  <li><code>.gitignore</code></li>
  <li><code>.golangci.yml</code></li>
  <li><code>config/config.yaml</code></li>
  <li><code>go.mod</code></li>
  <li><code>go.sum</code></li>
  <li><code>main.go</code></li>
  <li><code>migrations/00001_init_notes_and_reminders.go</code></li>
  <li><code>src/config/config.go</code></li>
  <li><code>src/database/cache/heap.go</code></li>
  <li><code>src/database/cache/reminder_cache.go</code></li>
  <li><code>src/database/cache/reminder_cache_test.go</code></li>
  <li><code>src/database/types.go</code></li>
  <li><code>src/logger/logger.go</code></li>
</ul>

<h2>v0.1.0 - создана базовая структура приложения</h2>

<h3>Что было сделано</h3>
<ul>
  <li>Были созданы Go-модуль и точка входа приложения.</li>
  <li>Был добавлен базовый загрузчик конфигурации с чтением YAML, значениями по умолчанию и валидацией.</li>
  <li>Были добавлены абстракция логгера и реализация на Zap.</li>
  <li>Была добавлена первая реализация кеша напоминаний.</li>
</ul>

<h3>Изменения API</h3>
<ul>
  <li>Публичный HTTP API и внешний API приложения не добавлялись.</li>
  <li>Были добавлены первые внутренние API конфигурации, логгера и кеша напоминаний.</li>
</ul>

<h3>Затронутые файлы</h3>
<ul>
  <li><code>.gitignore</code></li>
  <li><code>config/config.yaml</code></li>
  <li><code>go.mod</code></li>
  <li><code>go.sum</code></li>
  <li><code>main.go</code></li>
  <li><code>src/config/config.go</code></li>
  <li><code>src/database/cache/reminder_cache.go</code></li>
  <li><code>src/logger/api.go</code></li>
  <li><code>src/logger/logger.go</code></li>
</ul>
</div>
