#!/bin/bash

set -e

function str_to_array {
  eval "local input=\"\$$1\""
  input="$(echo "$input" | awk '
  {
    split($0, chars, "")
    for (i = 1; i <= length($0); i++) {
      if (i > 1) {
        printf(", ")
      }
      printf("\\\\\\\"%s\\\\\\\"", chars[i])
    }
  }
  ')"
  eval "$1=\"$input\""
}

function update_access_key {
  str_to_array API_PREFIX
  str_to_array BUCKET
  str_to_array ALIYUN_ACCESS_KEY
  str_to_array ALIYUN_ACCESS_SECRET
  awk "
  /DEFAULT_API_PREFIX/ {
    print \"var DEFAULT_API_PREFIX = strings.Join([]string{${API_PREFIX}}, \\\"\\\")\"
    next
  }
  /DEFAULT_BUCKET/ {
    print \"var DEFAULT_BUCKET = strings.Join([]string{${BUCKET}}, \\\"\\\")\"
    next
  }
  /KEY/ {
    print \"var KEY = strings.Join([]string{${ALIYUN_ACCESS_KEY}}, \\\"\\\")\"
    next
  }
  /SECRET/ {
    print \"var SECRET = strings.Join([]string{${ALIYUN_ACCESS_SECRET}}, \\\"\\\")\"
    next
  }
  {
    print
  }
  " access.go > _access.go

  mv _access.go access.go
}

_DEFAULT_API_PREFIX='https://%s.oss-cn-hangzhou.aliyuncs.com'
if test -z "$API_PREFIX"; then
  echo -n "Please enter default API domain: ($_DEFAULT_API_PREFIX if empty) "
  read API_PREFIX
  if test -z "$API_PREFIX"; then
    API_PREFIX=$_DEFAULT_API_PREFIX
  fi
fi
while test -z "$BUCKET"; do
  echo -n "Please enter default bucket name: "
  read BUCKET
done
while test -z "$ALIYUN_ACCESS_KEY"; do
  echo -n "Please paste your access key ID: (will not be echoed) "
  read -s ALIYUN_ACCESS_KEY
  echo
done
while test -z "$ALIYUN_ACCESS_SECRET"; do
  echo -n "Please paste your access key SECRET: (will not be echoed) "
  read -s ALIYUN_ACCESS_SECRET
  echo
done
update_access_key

if test -n "$BUILD_DOCKER"; then
  docker-compose up
  docker-compose rm --force -v
else
  go build
fi

API_PREFIX=$_DEFAULT_API_PREFIX
BUCKET="bucket"
ALIYUN_ACCESS_KEY="key"
ALIYUN_ACCESS_SECRET="secret"
update_access_key
