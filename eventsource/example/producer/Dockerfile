FROM golang:1.15
WORKDIR /app
COPY . /app
RUN go mod tidy
CMD ["go", "run", "main.go"]
