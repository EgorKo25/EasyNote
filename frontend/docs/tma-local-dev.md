# Локальный запуск EasyNote в Telegram Mini App

Пошаговый гайд: как открыть dev-версию web-сборки фронтенда внутри Telegram.

Telegram **не принимает** `http://localhost` — Mini App URL должен быть **публичным и HTTPS**. Локальный Metro/Expo поднимается у тебя на машине, а HTTPS-туннель проксирует его наружу.

## Что понадобится

- Node.js 20+ и установленные зависимости фронта (`npm install` в `frontend/`)
- Telegram-аккаунт
- Бот EasyNote (или любой тестовый бот), к которому у тебя есть доступ через [@BotFather](https://t.me/BotFather)
- Один из туннелей: **Expo tunnel** (проще всего) или **ngrok** / **cloudflared**

## Шаг 1. Запусти web dev server

```bash
cd frontend
npm run web
```

Дождись строки вроде:

```text
Waiting on http://localhost:8081
```

Запомни порт (обычно **8081**). Оставь терминал открытым — Metro должен работать, пока тестируешь Mini App.

Проверка в браузере: открой `http://localhost:8081` — должен открыться placeholder EasyNote Frontend.

## Шаг 2. Подними HTTPS-туннель

Выбери один способ.

### Вариант A — Expo tunnel (рекомендуется)

Останови предыдущий `npm run web` (Ctrl+C) и запусти:

```bash
cd frontend
npx expo start --web --tunnel
```

Expo создаст публичный HTTPS URL (часто вида `https://xxxx.exp.direct` или похожий). Скопируй **полный HTTPS URL** из вывода терминала.

> Первый запуск может попросить установить `@expo/ngrok` — соглашайся.

### Вариант B — ngrok

В **отдельном** терминале, пока Metro крутится на 8081:

```bash
ngrok http 8081
```

Скопируй **Forwarding** URL вида `https://abc123.ngrok-free.app`.

Бесплатный ngrok иногда показывает interstitial-страницу — для dev обычно ок; если Telegram не открывает app, попробуй Expo tunnel или cloudflared.

### Вариант C — cloudflared

```bash
cloudflared tunnel --url http://localhost:8081
```

Скопируй выданный `https://....trycloudflare.com` URL.

## Шаг 3. Настрой Mini App в BotFather

1. Открой [@BotFather](https://t.me/BotFather) в Telegram.
2. Выбери своего бота (или создай тестового: `/newbot`).
3. Настрой Mini App одним из способов:

**Способ 1 — Menu Button (удобно для dev):**

```text
/mybots → [твой бот] → Bot Settings → Menu Button → Configure menu button
```

- Укажи текст кнопки, например `Open EasyNote`
- В поле URL вставь **HTTPS URL из туннеля** (без лишнего пути, если Expo отдал корень)

**Способ 2 — Direct Link:**

```text
/mybots → [твой бот] → Bot Settings → Configure Mini App → Edit Mini App URL
```

Вставь тот же HTTPS URL.

**Способ 3 — команда (если поддерживается твоей версией BotFather):**

```text
/setmenubutton
```

Выбери бота → укажи URL.

> URL должен начинаться с `https://`. `http://`, `localhost`, IP без TLS — **не сработают**.

## Шаг 4. Открой Mini App в Telegram

1. Найди своего бота в Telegram (поиск по username).
2. Нажми **Start** / **Запустить**, если ещё не нажимал.
3. Открой Mini App:
   - через **кнопку меню** слева от поля ввода (если настроил Menu Button), или
   - через direct link, если BotFather выдал ссылку вида `https://t.me/YourBot/app`

Должен загрузиться твой локальный фронт через туннель. На placeholder-экране будет строка `Platform: Web (Telegram Mini App target)`.

## Шаг 5. Разработка с hot reload

- Меняешь код в `frontend/src/` → Metro пересобирает бандл.
- В Telegram иногда нужно **закрыть и снова открыть** Mini App, чтобы увидеть изменения (pull-to-refresh в WebView не всегда доступен).
- Если туннель перезапустился и URL **изменился** — обнови URL в BotFather (шаг 3).

## Частые проблемы

### «Mini App не открывается / белый экран»

- Проверь, что Metro всё ещё запущен (`npm run web` или `expo start --web --tunnel`).
- Открой HTTPS URL туннеля **в обычном браузере** — страница должна грузиться.
- Убедись, что в BotFather указан именно **https**, без опечаток и без trailing issues.
- Перезапусти Mini App в Telegram полностью (свайп вниз / закрыть и открыть снова).

### URL туннеля сменился

Expo tunnel и бесплатный ngrok часто выдают новый URL при каждом запуске. Обнови URL в BotFather.

Для стабильного dev-домена — платный ngrok с reserved domain или задеплоенный preview на постоянный HTTPS хост.

### `spawn adb ENOENT` / Android SDK

На Android-эмулятор это не влияет. Для TMA нужен только **web + HTTPS tunnel**, команда `npm run android` не требуется.

### CORS / API пока не работает

Сейчас фронт — placeholder без вызовов бэкенда. Когда появится HTTP API, для TMA понадобится валидация Telegram `initData` на бэкенде — см. [roadmap.md](./roadmap.md).

## Краткая шпаргалка

```bash
# Терминал 1 — dev server + tunnel
cd frontend
npx expo start --web --tunnel

# Скопировать HTTPS URL → BotFather → Menu Button / Mini App URL
# Telegram → открыть бота → нажать кнопку Mini App
```

## Дальше

- Интеграция `@telegram-apps/sdk-react` (theme, BackButton, initData) — [roadmap.md](./roadmap.md), Phase 3
- Прод-сборка для хостинга: `npm run build:web` → статика в `dist/`
