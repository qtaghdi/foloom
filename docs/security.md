# Local Security Model

The current service is a local single-user application, not a hosted identity
or tenant-isolation system. HTTP mode binds to loopback by default and uses
separate browser-session and MCP bearer credentials. Stdio browser fallback is
restricted to loopback and issues single-use launch links that expire after two
minutes.

## Credential Delivery

HTTP startup output is an intentional credential-delivery channel. The browser
URL and generated MCP bearer credential are printed once so the local user can
connect a browser or MCP client. Treat terminal output as sensitive, do not
copy it into issues or logs, and prefer explicitly supplied credentials when a
process supervisor records standard error. Routine HTTP responses, errors, and
application diagnostics must never repeat either credential.

Stdio keeps standard output protocol-only. Its `open_workbook` fallback returns
a short-lived single-use URL instead of the reusable browser credential. The
redirect removes the launch token from the address bar and sets an HTTP-only,
same-site cookie.

## Enforced Boundaries

- Browser routes require the browser session cookie.
- MCP over HTTP requires its distinct bearer credential.
- A browser credential cannot authorize MCP, and an MCP credential cannot
  authorize browser routes.
- Browser writes with a foreign `Origin` are rejected before request decoding
  or workbook mutation.
- Authentication failures do not expose credentials or workbook cell values.
- Browser responses apply a same-origin resource policy, deny framing, disable
  camera, microphone, and geolocation, and restrict scripts, connections,
  images, fonts, and workers with Content Security Policy.
- Launch links are random, single-use, time-limited, and bounded in number.

These controls protect the local process boundary. They do not provide hosted
OAuth, per-user permissions, remote TLS termination, tenant isolation, or a
durable audit log.

## Verification

`internal/transport/httpapi/security_test.go` contains the negative browser and
MCP credential matrix, foreign-origin no-mutation test, response non-disclosure
checks, security-header assertions, and invalid or expired launch-link tests.
The existing transport tests cover valid browser and MCP credentials, one-time
launch consumption, launch capacity, and loopback-only fallback listeners.
