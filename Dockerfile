FROM golang:1.21-alpine3.19 AS build

WORKDIR /build
COPY ./* /build
RUN CGO_ENABLED=0 go build -o EPGStation-file-deleter .

FROM gcr.io/distroless/static-debian12
COPY --from=build /build/EPGStation-file-deleter /
CMD ["/EPGStation-file-deleter"]
