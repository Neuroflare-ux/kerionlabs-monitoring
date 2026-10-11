#Stage ONE

FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download 
COPY . .
RUN CGO_ENABLED=0 go build -o /out/internships ./cmd/internships

#Stage 2 :Run
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/internships /app/internships
COPY cmd/internships/boards.txt /app/boards.txt
RUN mkdir -p /app/data
EXPOSE 8080
ENTRYPOINT ["/app/internships"]
CMD ["-boards", "/app/boards.txt", "-data", "/app/data/jobs.json", "-addr", ":8080"]
