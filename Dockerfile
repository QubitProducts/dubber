FROM golang:latest AS build

COPY go.mod go.sum *.go /src/
COPY cmd/root.go /src/cmd/root.go
COPY cmd/dubber/main.go /src/cmd/dubber/main.go
WORKDIR /src
RUN go get .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dubber ./cmd/dubber

FROM alpine
COPY --from=build  /dubber /
ENTRYPOINT ["/dubber"]
