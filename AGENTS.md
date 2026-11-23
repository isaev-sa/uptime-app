# Repository Guidelines

## Project Structure

The repository contains two projects: `frontend/` and `backend/`. See the `AGENTS.md` in each directory for project-specific commands and conventions.

## Testing

Run the relevant project's checks for each change. Add tests near the code they cover and document new test commands when introducing a test framework.

## Commit & Pull Request Guidelines

Git history was unavailable when this guide was written, so no established commit convention could be verified. Use concise, imperative commit subjects that describe the change. Pull requests should explain the purpose and behavior, list verification performed, link relevant issues, and include screenshots for visible UI changes.


## Коммиты
- Формат: Conventional Commits (feat, fix, refactor, test, docs, chore)
- Заголовок до 72 символов, в повелительном наклонении
- Без эмодзи, без "significantly improved" и прочей воды
- Тело - только если нужно объяснить "почему" а не "что"
- Один логический шаг - один коммит