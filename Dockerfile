FROM golang:1.27.1-alpine AS build-stage

WORKDIR /src

COPY backend/go.mod backend/go.sum ./backend/
RUN go -C backend mod download

COPY backend/ ./backend/

RUN CGO_ENABLED=0 GOOS=linux go -C backend build -o /server ./cmd/server
# distroless has no shell to mkdir with, so the directory is made here and copied over
RUN mkdir -p /out/backend


# deploy into a lean image
FROM gcr.io/distroless/static-debian12

WORKDIR /

COPY --from=build-stage /server /server
# the server writes its logs under backend/, relative to the repo root; see backend/CLAUDE.md
COPY --from=build-stage /out/ /

EXPOSE 8080

ENTRYPOINT ["/server"]
