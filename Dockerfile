# https://github.com/jtagcat/dotfiles/blob/main/scripts/template/gobuild.Dockerfile
FROM golang:1.23 AS builder
WORKDIR /wd

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o url-tocsv

FROM alpine
LABEL org.opencontainers.image.source="https://github.com/jtagcat/url-tocsv"
WORKDIR /wd
#RUN apk --no-cache add ca-certificates

COPY --from=builder /wd/url-tocsv ./
ENV OUTDIR=/wd/data
CMD ["./url-tocsv"]
