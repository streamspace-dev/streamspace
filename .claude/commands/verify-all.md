---
model: haiku
---

# Complete Pre-Commit Verification

Run all verification checks before committing code.

## API Backend
!cd api && go test ./... && go vet ./...

## UI
!cd ui && npm run lint && npm test -- --run

## K8s Agent
!cd agents/k8s-agent && go test ./...

## Docker Agent
!cd agents/docker-agent && go test ./...

## Success Criteria
- ✅ All tests passing (0 failures)
- ✅ No linting errors
- ✅ No vet warnings
- ✅ Build succeeds for all components

If any check fails:
1. Show which component failed
2. Display specific error messages
3. Suggest fixes based on error type
4. Offer to implement fixes if requested
5. DO NOT allow commit until all checks pass
