# Adding a New Locale

This guide explains how to add a new UI language to Monstera.

## Steps

### 1. Copy the English message file

```bash
cp ui/messages/en.json ui/messages/<locale>.json
```

For example, to add French:

```bash
cp ui/messages/en.json ui/messages/fr.json
```

Use a valid [BCP-47 language tag](https://www.iana.org/assignments/language-subtag-registry/language-subtag-registry) as the filename (e.g. `fr`, `de`, `zh-Hant`, `pt-BR`).

### 2. Translate the values

Open `ui/messages/<locale>.json` and translate all the string values. Do **not** change the keys. Keep ICU message syntax intact — for example `{field}`, `{appName}`, and `{username}` are interpolation placeholders that must remain as-is.

### 3. Register the locale

Open `ui/i18n/config.ts` and add the new locale to the `locales` array:

```ts
export const locales = ['en', 'fr'] as const;
```

### 4. Import the message bundle in the root layout

Open `ui/app/layout.tsx` and import the new message file, then add it to the `messages` map:

```tsx
import enMessages from '../messages/en.json';
import frMessages from '../messages/fr.json';

const messages = {
  en: enMessages,
  fr: frMessages,
};
```

### 5. Verify the build

```bash
cd ui && npm run build
```

TypeScript will catch any missing keys relative to the `en.json` baseline (via the `IntlMessages` global interface in `ui/types/next-intl.d.ts`).

### 6. Test the locale selector

1. Start the dev server: `cd ui && npm run dev`
2. Log in and go to **Account → Preferences**
3. Select the new locale from the **Language** dropdown
4. The page reloads; verify all strings appear in the new language
5. Check **DevTools → Application → Cookies** for `NEXT_LOCALE=<locale>`
6. Reload the page and confirm the locale persists

## Notes

- The locale is stored in a `NEXT_LOCALE` cookie (1-year max-age). There are no URL path changes.
- Only locales listed in `ui/i18n/config.ts` are accepted; unknown values fall back to `en`.
- Error message keys (in the `errors` namespace) map directly to the `code` field returned by the API — keep these keys unchanged.
