FROM golang:latest as go-builder
MAINTAINER Valentin Kuznetsov vkuznet@gmail.com

# tag to use
ENV TAG=0.0.02

# build procedure
ENV WDIR=/data
WORKDIR $WDIR
ARG CGO_ENABLED=0
RUN git clone https://github.com/vkuznet/wflow-dbs
WORKDIR $WDIR/wflow-dbs
RUN git checkout tags/$TAG -b build && make

# https://blog.baeke.info/2021/03/28/distroless-or-scratch-for-go-apps/
FROM alpine:3.16
# FROM gcr.io/distroless/static AS final
RUN mkdir -p /data
COPY --from=go-builder /data/wflow-dbs/wflow-dbs /data/
COPY --from=go-builder /data/wflow-dbs/static /data/static
CMD ["/data/wflow-dbs"]
