#!/bin/bash

imageName=$1
imageTag=$2

docker-squash -t imageTag $imageName