# OIDC REST authentication specification

`CLAUDE.md` is intentionally not edited by automation. This document records the REST authentication contract.

- `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID` are mandatory. Startup performs OIDC Provider Discovery and fails if either is absent or discovery fails.
- The verifier from `github.com/coreos/go-oidc/v3/oidc` validates issuer, JWKS-backed signature, expiration, and audience (`OIDC_CLIENT_ID`). No static-key or permissive REST fallback exists.
- `/healthz` remains unauthenticated. Every other REST endpoint requires an RFC 6750 `Authorization: Bearer <access-token>` header.
- `GET`/`HEAD` require `warehouse-ops-agent.read`; a `warehouse-ops-agent.write` scope also grants read. Mutating methods require `warehouse-ops-agent.write`.
- Missing/invalid tokens return RFC 6750 challenges plus RFC 7807 `application/problem+json` responses; insufficient scope returns `403` and an `insufficient_scope` challenge.
- Tokens and credentials are never logged.
