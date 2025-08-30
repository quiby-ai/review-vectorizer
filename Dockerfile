FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/app /app
COPY config.toml /

ARG PG_DSN
ARG OPENAI_API_KEY

ENV PG_DSN=$PG_DSN
ENV OPENAI_API_KEY=$OPENAI_API_KEY

USER nonroot

ENTRYPOINT ["/app"]
