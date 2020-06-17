FROM ubuntu:20.04

RUN apt update
RUN apt install -y wget
RUN wget https://github.com/gohugoio/hugo/releases/download/v0.72.0/hugo_0.72.0_Linux-64bit.deb
RUN dpkg -i hugo_0.72.0_Linux-64bit.deb

WORKDIR /srv/hugo

EXPOSE 1313

