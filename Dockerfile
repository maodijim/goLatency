FROM golang:1.22.5
LABEL authors="andy"

COPY . /app
WORKDIR /app
RUN go build -o main .
ENTRYPOINT ["/app/main"]