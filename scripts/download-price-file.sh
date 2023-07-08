#!/bin/bash

set -e

DATE="$(date -d now +%Y-%m-%d)"

if [[ $# > 0 ]]; then
    DATE="$1"
fi

PRICE_LIST_ARN="$(aws pricing list-price-lists --output json --service-code AmazonEC2 --effective-date $DATE --currency-code USD --region-code us-east-1 | jq -r '.PriceLists[0].PriceListArn')"

URL="$(aws pricing get-price-list-file-url --output json --price-list-arn $PRICE_LIST_ARN --file-format json | jq -r '.Url')"

TMP=$(mktemp)

curl -s "$URL" | zstd -c -T0 > $TMP

OUTPUT="$(dirname $0)/../data/ec2-price-list-$DATE.json.zst"
mv "$TMP" "$OUTPUT"
chmod a-w "$OUTPUT"

readlink -f $OUTPUT