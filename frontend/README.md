# EasyNote Frontend

Expo (React Native) client for **EasyNote** — a reminder app delivered as:

- **Telegram Mini App** (web build inside Telegram WebView)
- **iOS / Android** native apps from the same codebase

This folder is a self-contained frontend service inside the EasyNote monorepo. Treat it like a standalone repository for install, run, and build workflows.

## Prerequisites

- **Node.js** 20 LTS or newer
- **npm** (bundled with Node.js)
- **Mobile (optional):**
  - iOS: Xcode + iOS Simulator (macOS only)
  - Android: Android Studio + emulator
- **Telegram Mini App dev (optional):** HTTPS tunnel (e.g. [ngrok](https://ngrok.com/) or Expo tunnel) — Telegram requires HTTPS for Mini App URLs

## Install

```bash
cd frontend
npm install
cp .env.example .env
```

Edit `.env` when the backend HTTP API is available. Only variables prefixed with `EXPO_PUBLIC_` are exposed to the client bundle.

## Development

Start the Expo dev server (platform picker, QR code, hot reload):

```bash
npm start
```

Platform-specific shortcuts:

```bash
npm run web       # Web browser — primary target for Telegram Mini App development
npm run ios       # iOS Simulator
npm run android   # Android emulator
```

Check the local Expo environment:

```bash
npm run doctor
```

Lint:

```bash
npm run lint
```

## Production build

### Web (Telegram Mini App hosting)

Static export for hosting behind nginx, CDN, or a Docker static file server:

```bash
npm run build:web
```

Output directory: `dist/` (configured in `app.json` → `expo.web.output: static`).

### Mobile (optional, requires Expo Application Services)

Native store builds are not part of the initial scaffold. When ready:

```bash
npx eas-cli login
eas build:configure
eas build --platform ios
eas build --platform android
```

See [Expo EAS Build](https://docs.expo.dev/build/introduction/) for account and credential setup.

## Project structure

```
frontend/
├── src/
│   ├── app/           # Expo Router screens (file-based routing)
│   ├── components/    # Shared UI components
│   ├── constants/     # Theme and layout tokens
│   └── hooks/         # Platform hooks
├── assets/            # Icons, splash, images
├── docs/
│   ├── roadmap.md          # Integration and deployment roadmap
│   └── tma-local-dev.md    # Local Telegram Mini App dev guide (RU)
├── app.json           # Expo config (name, slug, platforms)
├── package.json
└── .env.example       # Public client env template
```

Routing entry: `src/app/_layout.tsx` and `src/app/index.tsx`.

## Backend integration

The Go backend currently exposes a **Telegram bot only** — there is no HTTP REST API yet.

- Planned API base URL (placeholder): `EXPO_PUBLIC_API_BASE_URL` from `.env`
- Integration phases, auth, and deploy steps: [docs/roadmap.md](./docs/roadmap.md)

Do **not** connect the frontend directly to PostgreSQL. Business rules (recurrence, reminders, planner) live in the Go backend.

## Telegram Mini App testing

**Full guide (RU):** [docs/tma-local-dev.md](./docs/tma-local-dev.md)

Quick version:

1. Run web dev server: `npm run web`
2. Expose it over HTTPS (Telegram requires HTTPS for Mini App URLs):
   - Expo tunnel: `npx expo start --web --tunnel`
   - Or ngrok: `ngrok http 8081` (port may vary — check Expo output)
3. In [@BotFather](https://t.me/BotFather), configure your bot's **Mini App URL** to the HTTPS URL
4. Open the Mini App from your bot in Telegram

Telegram WebApp SDK integration (`@telegram-apps/sdk-react`) is planned — see [docs/roadmap.md](./docs/roadmap.md).

## Related documentation

- [Expo documentation](https://docs.expo.dev/)
- [Expo Router](https://docs.expo.dev/router/introduction/)
- EasyNote backend: [../README.md](../README.md)
