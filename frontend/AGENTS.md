# Frontend Guidelines

## Structure

The Next.js App Router lives in `app/`: `page.js` defines the home page and `layout.js` defines the root layout and metadata. Put frontend assets in `public/` when needed. Do not commit generated `.next/` output or `node_modules/`.

## Development Commands

Run these commands from `frontend/`:

- `npm ci` installs dependencies pinned in `package-lock.json`.
- `npm run dev` starts the development server.
- `npm run build` builds the production application.
- `npm start` serves the production build.

## Style & Testing

Match the existing JavaScript style: two-space indentation, single-quoted strings, and semicolons. Use PascalCase for React components and Next.js App Router file names such as `page.js` and `layout.js`. No formatter, lint script, test framework, or coverage threshold is currently configured. Verify changes with a production build and appropriate manual checks.
