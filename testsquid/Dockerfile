FROM debian
RUN apt-get update && apt-get install -y squid

RUN echo http_access allow all >> /etc/squid/squid.conf
EXPOSE 3128/tcp

COPY squid.conf /etc/squid/squid.conf
