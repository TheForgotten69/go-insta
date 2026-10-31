# go-insta: agent notes

Private Instagram Android API client (app 453.0.0.55.79). Each call is read out of the
decompiled app and proven against the live API on a test account.

**Adding or fixing an Instagram call** (a new endpoint, a request Instagram answers
400/500, a field or header the captures disagree on): follow
[`docs/reverse-engineering.md`](docs/reverse-engineering.md).

## Conventions

- Request parameters are named structs with `json` tags, sent through
  `reqOptions.Form` (plain form, or the URL query on a GET) or `reqOptions.Signed`
  (`signed_body`). Maps hold only HTTP headers and keys that are dynamic ids.
- Endpoints live in `const.go`; GraphQL doc ids, root fields and friendly names in
  `graphql.go`. Read GraphQL results with `insta.graphQLRoot`; an empty result wraps
  `ErrEmptyResponse`.
- A call is done when its effect is read back live (`media/{id}/info`, the friendship
  status, a list). A 200 alone proves nothing.
- Live calls run as the test account in `login.session.json` (bobby, 42865264907),
  one live caller at a time (parallel agents stay offline), each write reversed and
  its leftovers deleted. Relationship tests target @instagram (25025320).
- `go run ./cmd/smoke [-writes] [-highlight] [-reel file.mp4]` is the live regression;
  give each new call a step there.
- Commit and push to `main`; record changes under `## [Unreleased]` in
  `CHANGELOG.md`; tag a release only when the user asks for it.
