FROM golang:1.15 as builder

WORKDIR /src

COPY . .

RUN go build -o k8smulticast .

FROM golang:1.15 as app

WORKDIR /app

COPY --from=builder /src/k8smulticast /app/k8smulticast

ENTRYPOINT ["/app/k8smulticast"]
