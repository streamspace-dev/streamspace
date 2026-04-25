# Fix Import Errors

Fix import errors in Go or TypeScript files.

Language: $ARGUMENTS (go or ts)

## For Go Files

Run Go import fixer:
!goimports -w .

Clean up module dependencies:
!go mod tidy

Verify compilation:
!go build ./...

Common fixes:
- Add missing imports
- Remove unused imports
- Organize imports (stdlib, external, internal)
- Update go.mod for new dependencies

## For TypeScript/React Files

Scan for missing imports in UI:
!cd ui && npm run lint 2>&1 | grep "is not defined"

Common import fixes:

### Material-UI Icons
```typescript
import { Cloud } from '@mui/icons-material';
import { CheckCircle, Error, Warning } from '@mui/icons-material';
```

### Material-UI Components
```typescript
import { Box, Typography, Button } from '@mui/material';
```

### React Hooks
```typescript
import { useState, useEffect, useCallback } from 'react';
```

### React Router
```typescript
import { useNavigate, useParams, Link } from 'react-router-dom';
```

After fixes:
- Remove unused imports
- Organize alphabetically
- Group by source (react, external, internal, relative)

## Verification

Run tests to ensure no regression:
!cd ui && npm test -- --run

Show files modified with import fixes.
