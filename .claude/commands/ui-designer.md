# UI Designer

Focus on clean, responsive UIs with clear and consistent styling when writing or reviewing React + shadcn + Tailwind code.

## Stack conventions

- **Components**: Prefer shadcn components from `@/components/ui` (Button, Card, Dialog, Input, Label, Tabs, etc.) over custom divs with raw Tailwind. Use `cn()` from `@/lib/utils` to merge class names.
- **Tokens**: Use semantic Tailwind tokens (e.g. `bg-primary`, `text-muted-foreground`, `border`, `ring-ring`) instead of hard-coded colors so theming and dark mode stay consistent.
- **Variants**: Use component variants (e.g. `Button` variant/size) rather than ad-hoc classes. Extend via `className` only when needed.

## Layout and spacing

- Use a consistent spacing scale: `gap-2`, `gap-4`, `gap-6`, `p-4`, `p-6` for rhythm. Avoid arbitrary values unless necessary.
- Prefer `flex` with `gap` for alignment; use `grid` when layout is clearly tabular or multi-column.
- Contain main content: `min-w-0 flex-1` on flex children that should shrink, and a max-width or padding wrapper for readable line length where appropriate.

## Responsiveness

- Design mobile-first: base styles for small screens, then `sm:`, `md:`, `lg:` for larger breakpoints.
- Use responsive utilities for visibility (`hidden md:block`), spacing (`p-4 md:p-6`), and layout (`flex-col md:flex-row`).
- Keep touch targets at least 44px where possible (`min-h-10`, `py-2 px-4` on interactive elements).

## Consistency

- Reuse existing patterns: same card structure (CardHeader, CardTitle, CardContent), same button hierarchy (primary for main action, outline/ghost for secondary).
- Typography: use `text-sm` for body, `font-medium` / `font-semibold` for emphasis, `text-muted-foreground` for secondary text. Avoid one-off font sizes.
- Borders and radius: stick to the design system (e.g. `rounded-md`, `rounded-xl` on cards). Use `border` with default border color unless a specific variant is needed.

## Forms and inputs

- Pair every input with a Label (use `Label` from ui). Use consistent spacing between form groups (`space-y-2` or `gap-4`).
- Show validation state via existing patterns (e.g. `aria-invalid`, destructive border/ring) and optional helper text.
- Keep form layout predictable: stacked on small screens, inline or grid on larger when it fits.

## Review checklist

When reviewing UI code, check:

- [ ] Uses design tokens and shadcn components instead of one-off Tailwind colors/markup
- [ ] Spacing and typography match the rest of the app
- [ ] Layout works on narrow viewports and doesn't overflow
- [ ] Interactive elements have clear focus and sufficient touch targets
- [ ] No duplicated layout or styling logic that could be a shared component or pattern

## Don't

- Don't hard-code hex or RGB colors; use semantic tokens.
- Don't mix arbitrary spacing (e.g. `p-[13px]`) with the design scale without reason.
- Don't build custom equivalents of existing shadcn components (buttons, cards, dialogs) unless the component doesn't exist or can't be composed.
- Don't leave large gaps or inconsistent alignment between related elements (e.g. misaligned form labels and inputs).
