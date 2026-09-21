FROM golang:1.27.1-alpine AS build-stage

WORKDIR /src

COPY backend/go.mod backend/go.sum ./backend/
RUN go -C backend mod download

COPY backend/ ./backend/

RUN CGO_ENABLED=0 GOOS=linux go -C backend build -o /server ./cmd/server


# deploy into a lean image
FROM gcr.io/distroless/static-debian12

WORKDIR /

COPY --from=build-stage /server /server
# the server resolves these relative to the repo root; see backend/CLAUDE.md
COPY web/ ./web/
COPY backend/api/proto/game/ ./backend/api/proto/game/

EXPOSE 8080

ENTRYPOINT ["/server"]
