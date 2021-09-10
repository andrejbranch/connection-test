FROM golang:1.17 as build
WORKDIR $GOPATH/connection-test
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=linux go build -o=/bin/connection-test ./main.go

FROM scratch
COPY --from=build /bin/connection-test /bin/connection-test
EXPOSE 8080
ENTRYPOINT ["/bin/connection-test"]
