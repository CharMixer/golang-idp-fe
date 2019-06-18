#!/bin/bash
set -e

CLIENT_ID=idp-fe
FILE=.idp-fe.yml

if [ -f "$FILE" ]
then
  echo "Config file $FILE already exists"
  exit 1
fi

HYDRA_ADMIN_URL="http://hydra:4445"
HYDRA_DOCKER_NETWORK="sso-examples_trusted"

# Create client credentials for idp-be service
cmd=$(docker run --rm -it \
  -e HYDRA_ADMIN_URL="${HYDRA_ADMIN_URL}"\
  --network "${HYDRA_DOCKER_NETWORK}" \
  oryd/hydra \
  clients create \
    --skip-tls-verify \
    --id $CLIENT_ID \
    --name identity-provider-backend \
    --grant-types client_credentials \
    --response-types token \
    --scope oauth.*,idp.*)

if [ "$?" -eq 0 ]
then
SECRET=$(echo -n "$cmd" | grep Secret: | sed 's/^.*\(Secret:.*\)/\1/g' | awk '{ printf $2 }' )
echo "client_id: $CLIENT_ID" > $FILE
echo "client_secret: $SECRET" >> $FILE
chmod 400 $FILE
else
  echo "Error: $?"
  exit $?
fi


