FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gaps-cli .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/gaps-cli /usr/local/bin/gaps-cli
ENV GAPS_HISTORY_GRADES_FILE="/var/data/grades-history.json"
ENTRYPOINT ["/usr/local/bin/gaps-cli"]
CMD ["scraper", "--interval", "300"]
