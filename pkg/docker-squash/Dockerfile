FROM python:3.7-alpine

WORKDIR /usr/src/app

RUN pip install docker-squash

COPY . .

CMD ["./squash-script.sh", "TODO_ImageName", "TODO_ImageTag"]
