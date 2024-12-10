FROM golang:latest AS compiling_stage
RUN mkdir -p /go/src/20.2
WORKDIR /go/src/20.2
ADD main.go .
ADD go.mod .
RUN go install .
 
FROM alpine:latest
LABEL version="1.0.0"
LABEL maintainer="Evgeniy <test@test.ru>"
WORKDIR /root/
COPY --from=compiling_stage /go/bin/20.2 .
ENTRYPOINT ./20.2