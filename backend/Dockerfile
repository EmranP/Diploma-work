FROM golang:1.25.5

WORKDIR /app

RUN apt update && apt install -y git

RUN go install github.com/air-verse/air@v1.64.5

COPY go.mod go.sum ./

RUN go mod download || go mod download

COPY . .

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]