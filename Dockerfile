FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/wintergate ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=build /out/wintergate /app/wintergate
COPY config/config.yml /app/config/config.yml

ENV PORT=1313
ENV GIN_MODE=release

EXPOSE 1313
ENTRYPOINT ["/app/wintergate"]