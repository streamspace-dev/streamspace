# Playwright Testing Plan for StreamSpace UI

This document outlines the comprehensive end-to-end (E2E) testing strategy for the StreamSpace UI using Playwright.

## 🎯 Testing Goals

- **Critical Path Coverage**: Ensure core user flows (Login -> Create Session -> Connect -> Logout) work flawlessly.
- **Resilience**: Verify error handling and recovery mechanisms.
- **Cross-Browser Compatibility**: Validate functionality across Chromium, Firefox, and WebKit.
- **Visual Regression**: Detect unintended UI changes.

## 🏗️ Test Structure

Tests will be organized in `ui/e2e` mirroring the page structure:

```
ui/e2e/
├── auth/
│   ├── login.spec.ts           # Login, logout, password reset
│   └── registration.spec.ts    # Sign up flows
├── core/
│   ├── dashboard.spec.ts       # Dashboard stats and widgets
│   ├── sessions.spec.ts        # Session lifecycle (create, list, delete)
│   ├── applications.spec.ts    # App catalog and launching
│   └── session-viewer.spec.ts  # VNC/Stream interaction
├── settings/
│   ├── profile.spec.ts         # User profile updates
│   └── security.spec.ts        # 2FA, password changes
├── admin/
│   ├── users.spec.ts           # User management
│   └── system.spec.ts          # System settings
└── flows/
    ├── new-user-onboarding.spec.ts  # Full onboarding walkthrough
    └── collaboration.spec.ts        # Sharing sessions
```

## 🧪 Test Scenarios

### 1. Authentication & Authorization (`auth/`)

- **Login**:
  - Valid credentials -> Redirect to Dashboard.
  - Invalid credentials -> Show error message.
  - Session persistence (reload page).
- **Logout**:
  - Click logout -> Redirect to Login -> Clear local storage/cookies.
- **Protected Routes**:
  - Access `/dashboard` without auth -> Redirect to Login.

### 2. Core Workflows (`core/`)

- **Dashboard**:
  - Verify stats load correctly.
  - Check "Recent Sessions" list.
- **Session Management**:
  - **Create**: Launch new session from template -> Verify "Provisioning" state -> Verify "Running" state.
  - **Connect**: Click "Connect" -> Verify VNC viewer loads (mock websocket if needed).
  - **Stop/Delete**: Terminate session -> Verify removal from list.
- **Applications**:
  - Filter/Search applications.
  - View application details modal.

### 3. Settings (`settings/`)

- **User Profile**:
  - Update display name/email.
  - Upload avatar (mock file upload).
- **Security**:
  - Change password.
  - Enable/Disable 2FA (if applicable).

### 4. Admin Portal (`admin/`)

- **User Management**:
  - List users.
  - Promote/Demote user roles.
- **System Health**:
  - View system metrics.

### 5. Edge Cases & Error Handling

- **Network Failure**: Simulate offline mode during session creation.
- **API Errors**: Mock 500 errors for list endpoints -> Verify "Retry" button appears.
- **Empty States**: Verify UI when no sessions/apps exist.

## 🛠️ Implementation Strategy

### Phase 1: Foundation (Current)

- [x] Install Playwright.
- [x] Configure base settings.
- [ ] Create shared fixtures (auth state, mock data).

### Phase 2: Critical Paths (Priority)

- [ ] Implement `auth/login.spec.ts`.
- [ ] Implement `core/sessions.spec.ts` (Create/Delete).

### Phase 3: Secondary Features

- [ ] Implement Dashboard and Settings tests.
- [ ] Implement Admin tests.

### Phase 4: Advanced

- [ ] Visual regression testing.
- [ ] Network interception/mocking for stability.

## 📝 Best Practices

- **Selectors**: Use user-facing locators (`getByRole`, `getByText`) over CSS selectors.
- **Isolation**: Each test should be independent (use `beforeEach` for setup).
- **Mocking**: Mock external API calls for consistent test data, but keep some "live" tests for integration verification.
- **Authentication**: Use `global-setup` to save auth state and reuse it to avoid logging in for every test.

## 🏃‍♂️ Running Tests

- **All Tests**: `/test-e2e`
- **Specific File**: `/test-e2e file=e2e/auth/login.spec.ts`
- **UI Mode**: `/test-e2e ui`
