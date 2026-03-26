# Next.js Development

Before any Next.js work, find and read the relevant doc in `ui/node_modules/next/dist/docs/`. Training data is outdated — the docs are the source of truth.

---

## i18n / Translations

The UI uses `next-intl` in **client-only mode** (static export; no middleware, no server APIs).

- Message files live in `ui/messages/<locale>.json`. All strings are namespaced: `common`, `nav`, `auth`, `account`, `home`, `admin`, `moderator`, `profile`, `errors`, `empty`, `footer`.
- `IntlProvider` (in `ui/components/intl-provider.tsx`) reads the `NEXT_LOCALE` cookie on mount and feeds the correct bundle to `NextIntlClientProvider`. It is mounted in `ui/app/layout.tsx`.
- Pages and components use `useTranslations('namespace')` — never hard-code English strings.
- To add a new locale, see `docs/adding-a-locale.md`.

---

## UI Error Handling Strategy

### API layer (`lib/api/`)

All API functions throw `ApiResponseError` (from `lib/api/errors.ts`) instead of `new Error(body.error)`. This preserves the `code` and `params` fields from the server response body so they can be mapped to translation keys.

```ts
// lib/api/errors.ts
export class ApiResponseError extends Error {
  code: string | undefined;
  params: Record<string, string> | undefined;
}

// Use the helper in every API function:
export async function throwApiError(res: Response): Promise<never> { ... }
```

**Do not** throw `new Error(body.error)` in API functions — doing so loses the `code` field.

### Hook layer (`hooks/`)

Hooks call `useTranslations('errors')` and pass the caught error to `translateApiError(t, err)` (from `lib/i18n/errors.ts`). This maps `err.code` to a translated string, falling back to `err.message` or `t('fallback')`.

```ts
const tErr = useTranslations('errors');
// in catch:
setError(translateApiError(tErr, err));
```

### Component/page layer

Components receive `error: string | null` from their hook or local state — they render it directly without any translation logic.

### Error key conventions

Keys in the `errors` namespace match the `code` field from the API response 1:1 (e.g., `not_found`, `validation_failed`, `session_expired`). When the server adds a new error code, add a matching key to `ui/messages/en.json`.
