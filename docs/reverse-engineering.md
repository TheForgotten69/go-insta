# Adding an Instagram call

jadx gives the shape of a request (endpoint, parameter names, value encodings); the live
API gives the truth (signed or plain form, response shape, server rules). Captures are
the third source, needed only where the app's own runtime matters (see the end).

Research material lives in `~/Documents/projects/research/instagram/` (`$R` below):
the APK (`com.instagram.android_453.0.0.55.79-*.apkm`), the captures
(`log_feed_*.txt`), and `jadx/` (string-table resolver, GraphQL id map).

## Steps

### 1. Find the request

**Captured?** Dump it, scrubbed:

```sh
go run ./cmd/httplog dump -match '<url or friendly-name regex>' $R/log_feed_*.txt
```

**Otherwise, jadx:**

```sh
W=$(mktemp -d) && cd $W
unzip -q $R/com.instagram.android_453*.apkm base.apk && unzip -q base.apk 'classes*.dex'
for f in classes*.dex; do strings -n 6 $f | rg -qF 'media/%s/edit_media/' && echo $f; done   # which dex
jadx -q -j 4 --no-res --no-debug-info -d out13 classes13.dex                             # ~2 min per dex
rg -lF 'media/%s/edit_media/' out13/sources                                              # the builder
$R/jadx/resolve.py out13/sources/X/C0VEZ.java                                            # with keys resolved
```

`resolve.py` replaces the obfuscated string-table lookups (`0000.A00(1575)`) with their
strings; its header says how to add a table it does not know yet (an unresolved lookup
prints as `?<table>/<n>`).

**GraphQL:** `$R/jadx/ig4a-instagram-schema_client_persist_ids.json` maps every operation
name to its doc id; a `schema` of `instagram` means `graphql_www`, `distillery` means
`graphql/query`. The variables come from the query's data-source class (search the
operation name in the decompiled sources).

Done when you have every key the builder sets, each with its setter (table below), and
the conditions under which the app sends it.

### 2. Implement

Follow the conventions in `AGENTS.md`: a named request struct, `Form` or `Signed` on
`reqOptions`, the endpoint in `const.go`, `graphQLRoot` for GraphQL. Model the values
exactly as the setter encodes them (`"true"` vs `"1"`, a JSON string vs raw JSON).
Default to `Signed` with an embedded `sessionIDs`; step 3 settles it.

### 3. Prove

- **Offline:** a request-shape test with the harness's `capture` helper, or a replay test
  from a fixture in `testdata/<area>.json` (scrubbed `httplog dump` output). It checks
  method, path and every key value you derived.
- **Live:** a probe in `zz_<area>_live_test.go` (`//go:build live`), run with
  `go test -tags live -run <Name> -v .`. Make the call, then read the state back and
  print it. Reverse every write and delete what it created.

Done when the effect is read back live, or the reason it cannot be (eligibility, needs a
second account) is recorded in `PLAN.md` and on the method.

### 4. Record

A step in `cmd/smoke`, an entry under `## [Unreleased]` in `CHANGELOG.md`, and for a call
with many options a field reference like `docs/reel-fields.md`.

## Reference

### Builder setters

| setter | sends |
|---|---|
| `AQR(key, s)` | string |
| `AQi(key, s)` | raw JSON (an object or array in the signed body) |
| `A0K(key, b)` | `"true"` / `"false"` |
| `A0L(key, b)` | `"1"` / `"0"` |
| `A0G` / `A0H` / `A0F` | int / long / float as a string |
| `A0J(key, s)` | string, only when not null |
| `A0M(fmt, args)` | the endpoint path |
| `0214.A0Q(b, key, s)` | `AQR`, then build |
| `0220.A0M(session)` | a new POST |

### Response field names

GraphQL parsers (Pando models) read each field by the Java `String.hashCode()` of its
name, so the decompiled parser shows numbers (`case 3599307:`), not names. Hash the
dex strings until one matches (`3599307` is `user`), or read the field names off one
live response.

### What only the live API tells you

- **Signed body or plain form**: decided in a method jadx cannot decompile. Start with
  `Signed`; when Instagram answers 400, try `Form`, and try the numeric pk in place of
  `<pk>_<owner>` (`clips/item/*` needs both).
- **Response shape**: open-source clients and parser classes guess; print the live body.
- **Server rules**: eligibility and preconditions show up only as errors or silent
  no-ops.

### Quirks found so far

- `configure_to_clips` with `async_publish` answers PENDING with an empty media; publish
  synchronously and retry on 202 "Transcode not finished yet."
- `trial_params` on a personal account: HTTP 500, and the Reel may still be published as
  a normal public Reel; check own posts after any failed publish.
- Mute answers 400 unless you follow the account.
- GraphQL: gzip is per query, the null-validation header per endpoint
  (`graphql/query` takes the eager-dict one), roots can be aliased `1$field(args)`.
- DMs: the app sends and marks seen over MQTT; the REST endpoints are legacy.
- The Waydroid emulator (NVIDIA venus mode) cannot encode video, so Reels cannot be
  captured from it.

### When you still need a capture

- The header profile and its drift between app configs: the harness tables in
  `replay_test.go` (`currentBuildOnly`, `currentBuildValues`, `renamedHeaders`,
  `skippedHeaders`, …) record each difference.
- Flows where the app's own runtime decides the bytes: login, attestation, the upload
  pipeline.
- Capture with `just capture <scenario>` in `$R` (see its README); `dav1d-guard.py`
  waits for the device itself.

### Shell on this machine

zsh has `noclobber`: overwrite with `>|`. `pkill -f <pattern>` also matches the shell
running it; kill by pid or an anchored pattern (`pgrep -f '^/path/to/python script'`).
