# Test UI Components

Run UI tests with coverage reporting.

!cd ui && npm test -- --coverage --run $ARGUMENTS

After running tests:
1. Show test results (passed/failed counts)
2. Report coverage percentages by file type
3. Identify components without tests
4. Suggest test improvements for low-coverage areas

If tests fail:
- Check for import errors (common: missing Material-UI icons)
- Fix component rendering issues
- Resolve mock setup problems
- Add missing test providers (Router, Theme, etc.)
