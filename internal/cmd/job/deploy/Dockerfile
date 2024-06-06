FROM golang:alpine

RUN apk add --update nodejs npm aws-cli
RUN npm install -g aws-cdk

ADD run.sh /bin/run.sh

CMD ["sh", "/bin/run.sh"]
