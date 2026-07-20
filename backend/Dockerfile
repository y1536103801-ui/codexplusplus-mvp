# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS build

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN test -n "$TARGETARCH" \
    && CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -trimpath -ldflags="-s -w" -o /out/codexppp-backend .

FROM node:24-bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends bubblewrap ca-certificates curl git openssh-client \
    && npm install -g @openai/codex \
    && useradd --create-home --shell /usr/sbin/nologin codexppp \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/codexppp-backend /usr/local/bin/codexppp-backend

ENV CODEXPPP_ADDR=0.0.0.0:8787
ENV CODEXPPP_DATA=/app/data/codexppp.json
ENV CODEXPPP_CODEX_COMMAND=codex
ENV HOME=/home/codexppp

EXPOSE 8787
USER codexppp

CMD ["codexppp-backend"]
