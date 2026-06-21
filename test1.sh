#!/bin/sh
go build -o maestro && probe -stdio ./maestro -args "-config=config-test.json" -call reference_list
