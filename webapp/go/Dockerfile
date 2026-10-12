FROM golang:1.18.4-bullseye

WORKDIR /tmp
ENV DEBIAN_FRONTEND=noninteractive
RUN sed -i -e 's|http://|https://|g' -e '/bullseye-security/d' -e '/bullseye-updates/d' /etc/apt/sources.list && apt-get update && \
  apt-get install -y wget gcc g++ make sqlite3 && \
  apt-get -y update && \
  apt-get -y install default-mysql-client

RUN useradd --uid=1001 --create-home isucon
USER isucon

RUN mkdir -p /home/isucon/webapp/go
WORKDIR /home/isucon/webapp/go
COPY --chown=isucon:isucon ./ /home/isucon/webapp/go/

ENV GOPATH=/home/isucon/tmp/go
ENV GOCACHE=/home/isucon/tmp/go/.cache

CMD ["go", "run", "./cmd/isuports/main.go"]
