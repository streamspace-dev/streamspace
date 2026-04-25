# Test E2E (Playwright)

Run end-to-end tests using Playwright.

**Use this when**: Verifying full user flows, UI interactions, and integration.

## Usage

```bash
/test-e2e [options]
```

## Options

- `ui`: Run in UI mode (interactive)
- `debug`: Run in debug mode
- `project=<name>`: Run specific project (chromium, firefox, webkit)
- `file=<path>`: Run specific test file

## Examples

- Run all tests:

  ```bash
  /test-e2e
  ```

- Run in UI mode:

  ```bash
  /test-e2e ui
  ```

- Run specific file:

  ```bash
  /test-e2e file=e2e/example.spec.ts
  ```

## Execution

!cd ui && npm run test:e2e -- $ARGUMENTS
