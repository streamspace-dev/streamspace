# Test Go Packages

Run Go tests for the specified package or all packages if none specified.

!cd api && go test $ARGUMENTS -v -coverprofile=coverage.out -covermode=atomic

After running tests:
1. Show test results summary
2. Calculate coverage percentage using: `go tool cover -func=coverage.out | grep total`
3. Identify untested packages (0% coverage)
4. Suggest areas needing tests based on recent code changes

If tests fail:
- Analyze failure messages
- Identify root cause (compilation errors, assertion failures, etc.)
- Suggest fixes with specific line numbers
- Offer to implement fixes if requested
