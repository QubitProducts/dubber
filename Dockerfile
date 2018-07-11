FROM golang:1.10
COPY .  /go/src/github.com/QubitProducts/dubber
RUN go build -o /dubber github.com/QubitProducts/dubber/cmd/dubber
ENTRYPOINT ["/dubber"]

