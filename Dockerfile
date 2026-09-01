FROM golang:alpine AS build

RUN apk add --no-cache git gcc musl-dev upx
RUN mkdir /app
WORKDIR /app
COPY . .
RUN ln -s /usr/bin/gcc /usr/bin/musl-gcc && ./build-static.sh
RUN upx --best --lzma ./nvdaRemoteServer

FROM scratch

COPY --from=build /app/nvdaRemoteServer /nvdaRemoteServer
COPY --from=build /app/cert.pem /cert.pem

EXPOSE 6837
CMD ["/nvdaRemoteServer", "-configfile", "", "-certfile", "/cert.pem", "-keyfile", "/cert.pem"]
