#!/bin/bash

docker-compose down
docker volume rm ${PWD##*/}_evr-db
docker volume rm ${PWD##*/}_ox-db
