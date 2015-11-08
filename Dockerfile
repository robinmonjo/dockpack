FROM gliderlabs/alpine

RUN apk --update add curl git && mkdir /sandbox

ADD dockpack dockpack
ADD id_rsa id_rsa

ENTRYPOINT ["/dockpack"]
