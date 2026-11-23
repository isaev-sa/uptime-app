FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.21
RUN adduser -D -H app && mkdir -p /uploads/avatars && chown -R app:app /uploads
USER app
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
