FROM golang:1.21 as build
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o serverd main.go

FROM gcr.io/distroless/static-debian11
ARG BUILD_TAGS
ENV BUILD_TAGS=${BUILD_TAGS}

WORKDIR /app
COPY --from=build /app/serverd /app

CMD ["/app/serverd"]

