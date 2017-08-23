FROM scratch
COPY cmd/dubber/dubber /
EXPOSE 8080
ENTRYPOINT ["/dubber"]
