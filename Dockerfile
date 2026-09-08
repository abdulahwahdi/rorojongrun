# Stage 1
FROM golang:1.25.0-alpine3.21 AS dependency_builder

WORKDIR /go/src
ENV GO111MODULE=on

RUN apk update
RUN apk add --no-cache bash ca-certificates git make gcc musl-dev

COPY go.mod .
COPY go.sum .

RUN go mod download

# Stage 2
FROM dependency_builder AS service_builder

ARG SERVICE_NAME
WORKDIR /usr/app

COPY sdk sdk
COPY services/$SERVICE_NAME services/$SERVICE_NAME
COPY go.mod .
COPY go.sum .
RUN go build -o bin services/$SERVICE_NAME/*.go

# Stage 3
FROM alpine:3.21

ARG BUILD_NUMBER
ARG SERVICE_NAME
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /usr/app/
ENV WORKDIR=services/$SERVICE_NAME/
ENV BUILD_NUMBER=$BUILD_NUMBER

RUN mkdir -p /usr/app/services/$SERVICE_NAME
COPY --from=service_builder /usr/app/bin bin
COPY --from=service_builder /usr/app/services/$SERVICE_NAME/.env /usr/app/services/$SERVICE_NAME/.env

ENTRYPOINT ["./bin"]
