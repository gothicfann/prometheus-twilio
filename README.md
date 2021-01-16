# prometheus-twilio 
[![Build Status](https://travis-ci.com/gothicfann/prometheus-twilio.svg?branch=master)](https://travis-ci.com/gothicfann/prometheus-twilio)
![Docker Pulls](https://img.shields.io/docker/pulls/gothicfan/prometheus-twilio)

## Description
This service can accept alertmanager webhook requests and send SMSes via Twilio.

## Build
To build this project run: 
```shell
make build
```

You need to setup following environment variables before you run application:

```shell
TWILIO_ACCOUNT_SID      # Account SID
TWILIO_AUTH_TOKEN       # Auth Token
TWILIO_SENDER           # Twilio Sender Phone Number
```

## Usage
Use comma separated numbers in `recipients` query parameter.  
Make sure to have `summary` and `description` annotations in your prometheus alerting rules included.  
Example curl request to test:
```
curl --location --request POST 'localhost:8080/alert/send?recipients=995595951230,995595951231,995595951232' \
--header 'Content-Type: application/json' \
--data-raw '{
    "version": "2",
    "status": "firing",
    "alerts": [
        {
            "annotations": {
                "summary": "Summary1",
                "description": "Description1"
            },
            "startsAt": "2016-03-19T05:54:01Z"
        },
        {
            "annotations": {
                "summary": "Summary2",
                "description": "Description2"
            },
            "startsAt": "2016-03-19T05:54:01Z"
        }
    ]
}'
```