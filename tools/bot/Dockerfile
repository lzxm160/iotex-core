FROM golang:1.12.5-stretch

WORKDIR apps/iotex-bot

ENV http_proxy socks5://192.168.1.2:1080

RUN apt-get install -y --no-install-recommends make
RUN git clone -b 14165 https://github.com/lzxm160/iotex-core
RUN git config --global http.proxy 'socks5://192.168.1.2:1080' && \
    git config --global https.proxy 'socks5://192.168.1.2:1080'
RUN cd iotex-core/tools/bot && \
    make build && \
    cp ./bot /usr/local/bin/bot  && \
    mkdir -p /etc/iotex/ && \
    rm -rf apps/iotex-bot/

CMD [ "bot -config-path=/etc/iotex/config.yaml"]
