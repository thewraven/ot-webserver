FROM golang:1.15.5
RUN apt update && apt install -y memcached
COPY . /opt/webserver
ENV SERVICE_NAME mathService
ENV HONEYCOMB_KEY YOU_MUST_PROVIDE_YOUR_KEY
WORKDIR /opt/webserver
RUN go build -o server main.go 
RUN go build -o ot-client client/main.go
RUN chmod +x start.sh
ENTRYPOINT /opt/webserver/start.sh
