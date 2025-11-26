# Acceptance Criteria Guide

**Version**: v1.0
**Last Updated**: 2025-11-26
**Owner**: Architect + Product
**Status**: Living Document

---

## Introduction

This guide provides templates and examples for writing clear, testable acceptance criteria for StreamSpace features. Good acceptance criteria ensure shared understanding between product, engineering, and QA.

**Purpose**:
- Define "done" for features and user stories
- Enable effective testing (manual and automated)
- Reduce ambiguity and rework
- Facilitate estimation and planning

---

## Acceptance Criteria Format

### Standard Format: Given-When-Then

**Template**:
```
Given [precondition/context]
When [action/trigger]
Then [expected outcome/result]
```

**Why This Format?**
- **Clear**: Separates context, action, outcome
- **Testable**: Maps directly to test scenarios
- **Universal**: Works for unit, integration, E2E tests

---

## Examples by Feature Type

### 1. API Endpoint

**Feature**: Create Session API

**User Story**:
```
As a developer
I want to create a session via REST API
So that I can provision containerized applications programmatically
```

**Acceptance Criteria**:

```
✅ AC1: Successful Session Creation

Given I am authenticated with valid JWT token
  And my org has available quota (sessions < max_sessions)
  And template "ubuntu-desktop" exists
When I POST to /api/v1/sessions with:
  {
    "template_id": "ubuntu-desktop",
    "resources": {"cpu": "2", "memory": "4Gi"}
  }
Then I receive 201 Created response
  And response body contains session object with:
    - session_id (UUID)
    - status: "pending"
    - user_id: <my user ID>
    - org_id: <my org ID>
    - template_id: "ubuntu-desktop"
    - created_at (ISO 8601 timestamp)
  And session is inserted into database with status="pending"
  And command is dispatched to agent via WebSocket
```

```
❌ AC2: Quota Exceeded

Given I am authenticated
  And my org quota is 10 sessions
  And there are already 10 running sessions for my org
When I POST to /api/v1/sessions with valid payload
Then I receive 429 Too Many Requests response
  And response body contains:
    {
      "error": "Quota exceeded",
      "quota_limit": 10,
      "current_usage": 10
    }
  And no session is created
  And no command is dispatched to agent
```

```
❌ AC3: Invalid Template

Given I am authenticated
  And template "nonexistent-template" does not exist
When I POST to /api/v1/sessions with:
  {"template_id": "nonexistent-template"}
Then I receive 404 Not Found response
  And response body contains {"error": "Template not found"}
  And no session is created
```

```
❌ AC4: Unauthorized Access

Given I am NOT authenticated (no JWT token)
When I POST to /api/v1/sessions with valid payload
Then I receive 401 Unauthorized response
  And no session is created
```

```
❌ AC5: Org Scoping (Security)

Given I am authenticated as user in org "org-A"
  And template "restricted" exists in org "org-B" only
When I POST to /api/v1/sessions with template_id="restricted"
Then I receive 404 Not Found response (cross-org access blocked)
  And no session is created
```

---

### 2. UI Component

**Feature**: Session Card Component

**User Story**:
```
As a user
I want to see session details in a card
So that I can quickly identify and connect to my sessions
```

**Acceptance Criteria**:

```
✅ AC1: Display Session Information

Given a session object:
  {
    "id": "sess-123",
    "template_name": "Ubuntu Desktop",
    "status": "running",
    "created_at": "2025-11-26T10:00:00Z",
    "vnc_url": "wss://..."
  }
When SessionCard component is rendered
Then the card displays:
  - Session ID: "sess-123"
  - Template name: "Ubuntu Desktop"
  - Status badge: "Running" (green)
  - Created time: "2 hours ago" (relative format)
  - "Connect" button (enabled)
  - "Delete" button (enabled)
```

```
✅ AC2: Status Badge Colors

Given different session statuses
When SessionCard is rendered
Then status badges use correct colors:
  - "running": green (#4caf50)
  - "pending": yellow (#ff9800)
  - "stopped": gray (#9e9e9e)
  - "failed": red (#f44336)
```

```
✅ AC3: Connect Button Action

Given session status is "running"
When user clicks "Connect" button
Then onConnect callback is called with session.id
  And VNC modal opens with session VNC stream
```

```
❌ AC4: Connect Button Disabled

Given session status is "pending" or "stopped" or "failed"
When SessionCard is rendered
Then "Connect" button is disabled (grayed out)
  And button tooltip says "Session not ready"
```

```
✅ AC5: Delete Confirmation

Given session is displayed
When user clicks "Delete" button
Then confirmation dialog appears with:
  "Are you sure you want to delete session sess-123?"
  And [Cancel] and [Delete] buttons
When user clicks [Delete]
Then onDelete callback is called with session.id
  And session card is removed from UI
```

---

### 3. Business Logic / Service

**Feature**: Session Hibernation

**User Story**:
```
As a system admin
I want idle sessions to automatically hibernate
So that I can reduce infrastructure costs
```

**Acceptance Criteria**:

```
✅ AC1: Detect Idle Session

Given a session has been running for 30 minutes
  And there has been no VNC activity (no mouse/keyboard input) for 15 minutes
  And hibernation is enabled for the org
When the idle detection cron job runs
Then the session is marked as "idle" in database
  And a "hibernate_session" command is dispatched to the agent
```

```
✅ AC2: Hibernate Session

Given a "hibernate_session" command is received by agent
  And session pod is running
When agent processes the command
Then agent pauses the session container (SIGSTOP)
  And session status is updated to "hibernated" in database
  And session storage volume is retained (not deleted)
  And VNC connection is terminated gracefully
  And command status is marked "completed"
```

```
✅ AC3: Resume Hibernated Session

Given a session is in "hibernated" status
When user requests to connect (GET /api/v1/sessions/:id/vnc)
Then API dispatches "resume_session" command to agent
  And agent un-pauses the container (SIGCONT)
  And session status is updated to "running"
  And VNC token is generated and returned to user
  And user can connect within 60 seconds
```

```
❌ AC4: Hibernation Disabled

Given an org has hibernation disabled in settings
When idle detection cron job runs
Then no sessions for that org are hibernated
  And sessions continue running until manually stopped
```

```
✅ AC5: Hibernation Timeout

Given a session has been hibernated for 7 days
  And hibernation_max_duration is set to 7 days
When the cleanup cron job runs
Then the session is automatically deleted
  And all resources (pod, volume, CRD) are removed
  And user receives email notification "Session sess-123 deleted (hibernation timeout)"
```

---

### 4. Security Feature

**Feature**: Multi-Tenancy Org Scoping

**User Story**:
```
As a platform admin
I want sessions to be org-scoped
So that users in org A cannot access sessions in org B
```

**Acceptance Criteria**:

```
✅ AC1: JWT Contains org_id

Given a user authenticates via SSO
When JWT token is generated
Then token claims include:
  {
    "user_id": "user-123",
    "org_id": "org-abc",
    "role": "user"
  }
  And token is signed with platform secret
  And token expiry is 1 hour from issue time
```

```
✅ AC2: Session List Org-Scoped

Given I am authenticated as user in "org-A"
  And there are 5 sessions in "org-A"
  And there are 3 sessions in "org-B"
When I GET /api/v1/sessions
Then I receive only the 5 sessions from "org-A"
  And sessions from "org-B" are not returned
  And database query includes WHERE clause: org_id = 'org-A'
```

```
❌ AC3: Cross-Org Access Denied

Given I am authenticated as user in "org-A"
  And session "sess-999" exists in "org-B"
When I GET /api/v1/sessions/sess-999
Then I receive 404 Not Found response (not 403, to avoid enumeration)
  And no session details are returned
```

```
✅ AC4: WebSocket Broadcasts Org-Scoped

Given I am connected to WebSocket /ws/ui
  And I am in "org-A"
  And a session in "org-B" changes status to "running"
When WebSocket broadcast occurs
Then I do NOT receive the status update for org-B session
  And only users in "org-B" receive the update
```

```
✅ AC5: Admin Cross-Org Access

Given I am authenticated as platform admin (role="admin")
  And admin_cross_org_access feature flag is enabled
When I GET /api/v1/sessions?org_id=org-B
Then I receive sessions from "org-B" (admin override)
  And audit log records cross-org access:
    {
      "action": "list_sessions",
      "user_id": "admin-456",
      "target_org_id": "org-B",
      "reason": "admin override"
    }
```

---

## Checklist for Good Acceptance Criteria

Use this checklist when writing acceptance criteria:

### ✅ Clarity
- [ ] Uses Given-When-Then format
- [ ] Unambiguous language (no "maybe", "should", "probably")
- [ ] Specific values/examples provided
- [ ] No technical jargon (or explained if necessary)

### ✅ Testability
- [ ] Can be verified with automated test
- [ ] Measurable outcomes (response code, field values, state changes)
- [ ] Edge cases covered (happy path + error cases)

### ✅ Completeness
- [ ] Covers happy path (successful operation)
- [ ] Covers error cases (validation, auth, quota)
- [ ] Covers edge cases (empty input, max limits, timeouts)
- [ ] Defines both positive and negative tests

### ✅ Independence
- [ ] AC can be verified independently
- [ ] No dependencies on other unrelated features
- [ ] Self-contained preconditions

### ✅ Security
- [ ] Authentication/authorization verified
- [ ] Org scoping enforced (multi-tenancy)
- [ ] Input validation covered
- [ ] Sensitive data handling specified

---

## Anti-Patterns

### ❌ Vague Criteria

**Bad**:
```
When user creates a session
Then it should work
```

**Good**:
```
Given authenticated user with available quota
When user POSTs to /api/v1/sessions with valid template_id
Then API returns 201 Created with session object
  And session status is "pending"
  And command is dispatched to agent
```

---

### ❌ Implementation Details

**Bad**:
```
Given database connection is established
When SessionRepository.Insert() is called with session object
Then row is inserted into sessions table using SQL INSERT statement
```

**Good**:
```
Given valid session creation request
When session is created
Then session is persisted with status "pending"
  And session can be retrieved via GET /api/v1/sessions/:id
```

---

### ❌ Missing Error Cases

**Bad** (only happy path):
```
When user creates session
Then session is created
```

**Good** (happy + error cases):
```
✅ When user creates session with valid data → 201 Created
❌ When user creates session with invalid template → 404 Not Found
❌ When user exceeds quota → 429 Quota Exceeded
❌ When unauthenticated user creates session → 401 Unauthorized
```

---

### ❌ Non-Testable Criteria

**Bad**:
```
The system should be fast
```

**Good**:
```
When session creation request is made
Then API responds within 200ms (p95)
  And session provisioning completes within 30 seconds (p95)
```

---

## Estimation Using Acceptance Criteria

Use acceptance criteria to estimate story points:

**T-Shirt Sizing**:
- **XS** (1 point): 1-2 acceptance criteria, straightforward logic
- **S** (2 points): 3-4 acceptance criteria, simple validation
- **M** (3 points): 5-7 acceptance criteria, moderate complexity
- **L** (5 points): 8-10 acceptance criteria, complex business logic
- **XL** (8 points): 10+ acceptance criteria, requires design review

**Example**:
- Session creation API (5 AC) = **M** (3 points)
- Multi-tenancy org scoping (7 AC) = **M-L** (4 points)
- Session hibernation (5 AC) = **M** (3 points)

---

## From AC to Tests

### Mapping AC to Test Cases

**Acceptance Criterion**:
```
Given authenticated user in org "org-A"
When user POSTs to /api/v1/sessions with template_id="ubuntu"
Then API returns 201 Created
  And response contains session with status="pending"
  And session.org_id = "org-A"
```

**Test Case** (Go):
```go
func TestCreateSession_Success(t *testing.T) {
    // Given: authenticated user in org-A
    ctx := context.WithValue(context.Background(), "user_id", "user-123")
    ctx = context.WithValue(ctx, "org_id", "org-A")

    req := CreateSessionRequest{
        TemplateID: "ubuntu",
    }

    // When: user creates session
    session, err := handler.CreateSession(ctx, req)

    // Then: session created with status pending
    assert.NoError(t, err)
    assert.Equal(t, "pending", session.Status)
    assert.Equal(t, "org-A", session.OrgID)
    assert.Equal(t, "ubuntu", session.TemplateID)
}
```

---

## References

- **Given-When-Then**: [Cucumber Documentation](https://cucumber.io/docs/gherkin/reference/)
- **User Story Mapping**: [Jeff Patton's Story Mapping](https://www.jpattonassociates.com/user-story-mapping/)
- **BDD**: [Behavior-Driven Development](https://dannorth.net/introducing-bdd/)

---

## Templates

### API Endpoint Template

```markdown
## Feature: [Endpoint Name]

**User Story**:
As a [role]
I want to [action]
So that [benefit]

**Acceptance Criteria**:

✅ AC1: Successful [Operation]
Given [preconditions]
When [action]
Then [expected outcome]

❌ AC2: [Error Case 1]
Given [preconditions]
When [action]
Then [expected error response]

❌ AC3: [Error Case 2]
...
```

### UI Component Template

```markdown
## Component: [Component Name]

**User Story**:
As a [user role]
I want to [interact with component]
So that [benefit]

**Acceptance Criteria**:

✅ AC1: Display [Data/State]
Given [props/state]
When component is rendered
Then [visible elements]

✅ AC2: [User Interaction]
Given [initial state]
When user [action]
Then [expected behavior]

❌ AC3: [Error/Edge Case]
...
```

---

**Version History**:
- **v1.0** (2025-11-26): Initial acceptance criteria guide
- **Next Review**: v2.1 release (Q1 2026)
