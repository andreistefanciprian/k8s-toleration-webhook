FROM golang:1.19-alpine AS build

WORKDIR /app

COPY . .

RUN go mod download

ENV GOOS=linux \
GOARCH=386

RUN go build -a -o webhook

## Deploy
FROM gcr.io/distroless/base-debian11
# FROM alpine

COPY --from=build /app/webhook .

# USER nonroot:nonroot

ENTRYPOINT ["./webhook"]