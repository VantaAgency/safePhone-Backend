# Project Management

This repository uses GitHub Issues to track work.

## Issue types
- `feature`: a new capability, endpoint, integration, or domain flow
- `bug`: something broken, incorrect, or unstable
- `improvement`: refinement to performance, security, architecture, DX, or maintainability

## Recommended labels
### Type
- `feature`
- `bug`
- `improvement`

### Priority
- `priority: high`
- `priority: medium`
- `priority: low`

### Area
- `area: api`
- `area: auth`
- `area: claims`
- `area: payments`
- `area: notifications`
- `area: admin`
- `area: employee`
- `area: database`
- `area: infrastructure`

### Severity
- `severity: critical`
- `severity: major`
- `severity: minor`

## Suggested workflow
1. Create an issue with the correct template.
2. Add labels for type, priority, and area.
3. Add the issue to your GitHub Project board.
4. Move the issue through: `Backlog -> Planned -> In Progress -> Review -> Done`.
5. Link pull requests to issues with `Fixes #issue_number`.

## SafePhone usage
Use this repository primarily for backend and platform work such as:
- API endpoints
- authentication and authorization
- claims processing
- payments and integrations
- database changes
- notifications and background jobs

Keep product/UI issues in the frontend repository when they are implementation-specific to the client app.
