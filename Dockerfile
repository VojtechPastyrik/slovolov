FROM node:26-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/daily ./cmd/daily

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/daily /app/daily
COPY --from=web /web/dist /app/web/dist
ENV STATIC_DIR=/app/web/dist HTTP_ADDR=:8080 PUZZLE_TIMEZONE=Europe/Prague
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
