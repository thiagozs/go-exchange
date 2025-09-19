FROM golang:1.24.7-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go env -w GO111MODULE=on
RUN go mod download
COPY . .
RUN go build -o /app/bin/go-exchange ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /app/bin/go-exchange /usr/local/bin/go-exchange
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/go-exchange"]
