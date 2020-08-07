FROM node:10

WORKDIR /build

RUN git clone https://github.com/nodejs/nodejs.org.git; cd /build/nodejs.org; npm  install

WORKDIR /build/nodejs.org

ENTRYPOINT [ "/usr/local/bin/npm", "start" ]