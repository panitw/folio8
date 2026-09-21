# folio8 Designer — production image for Railway.
#
# The designer is a static page, but its build compiles the Go engine to wasm
# (build:wasm), so the build stage carries both toolchains at the exact versions
# the repo pins: Go 1.26.0 (folio-go/go.mod toolchain, AD-22) and Node 24.16.0
# (folio-designer/package.json engines).
# The context is the repo root: build-wasm.mjs reads ../folio-go and ../docs.

FROM golang:1.26.0-bookworm AS go

FROM node:24.16.0-bookworm AS build
COPY --from=go /usr/local/go /usr/local/go
# GOTOOLCHAIN=local: never let `go` fetch a different toolchain mid-build.
ENV PATH=/usr/local/go/bin:$PATH \
    GOTOOLCHAIN=local
WORKDIR /src

COPY folio-designer/package.json folio-designer/package-lock.json folio-designer/
RUN cd folio-designer && npm ci

COPY folio-go/go.mod folio-go/go.sum folio-go/
RUN cd folio-go && go mod download

COPY . .

# USAGE MEASUREMENT (spec-google-analytics, AD-27). The GTM container id is a
# BUILD-TIME input — Vite substitutes it into the bundle — so it must reach
# `vite build` as an environment variable of THIS stage; a runtime service
# variable on the Caddy image arrives far too late and the deployed page would
# be silently inert. Unset (the default) means no script, no dataLayer and no
# event, which is exactly what a local `docker build` should produce.
#
# The operator sets it on the Railway service as a BUILD variable; see
# RELEASING.md, "Usage measurement in the deployed designer".
ARG VITE_GA_CONTAINER_ID=
ENV VITE_GA_CONTAINER_ID=$VITE_GA_CONTAINER_ID

# `npm run build` minus its first two steps. scan:font-hosts and scan:host-fonts
# are source guardrails that populate from `git ls-files` and refuse to run
# without a checkout — and neither a Railway upload nor this build context
# carries .git. They do not shape the output and CI runs them on every push.
# Everything that does shape or verify the release runs, in the same order.
RUN cd folio-designer \
 && npm run build:wasm \
 && npx tsc -b \
 && npx vite build \
 && npm run build:offline \
 && npm run verify:offline

# Pinned exactly: Caddy 2.10.2 answers a brotli-precompressed GET with 206
# instead of 200, and Cache.put() rejects 206, which breaks the service
# worker's offline precache. 2.11.4 answers 200 (measured).
FROM caddy:2.11.4-alpine
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /src/folio-designer/dist /srv
