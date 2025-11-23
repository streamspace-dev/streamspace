# Generate Semantic Commit Message

Analyze staged changes and create a semantic commit message following StreamSpace conventions.

!git diff --staged

Generate commit message with this format:

```
<type>(<scope>): <subject>

<body>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## Type Options
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding/updating tests
- `refactor`: Code refactoring
- `chore`: Maintenance tasks
- `perf`: Performance improvements

## Scope Options
- `api`: API backend changes
- `k8s-agent`: Kubernetes agent
- `docker-agent`: Docker agent
- `ui`: Frontend/UI changes
- `architect`: Architect agent work
- `builder`: Builder agent work
- `validator`: Validator agent work
- `scribe`: Scribe agent work
- `infra`: Infrastructure/deployment

## Subject Guidelines
- Clear, concise summary (50 chars max)
- Imperative mood ("Add feature" not "Added feature")
- No period at the end

## Body Guidelines
- Bullet points for significant changes
- Explain WHY not WHAT (code shows what)
- Reference issue numbers (#123)
- Note breaking changes

**IMPORTANT**: DO NOT commit automatically. Show the generated message for user review and approval first.
