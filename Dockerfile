FROM golang:1.23-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /yokai

FROM alpine

WORKDIR /

COPY --from=build /yokai /yokai

ENTRYPOINT ["/yokai"]
