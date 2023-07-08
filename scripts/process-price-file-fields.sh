#!/bin/bash

FILE="$1"

if [ -z "$FILE" ]; then
    echo "pass the price file as argument. Price files can be downloaded with download-price-file.sh"
    exit 2
fi

DATA_DIR="$(readlink -f $(dirname $0)/../data/)"

ATTRIBUTES="$DATA_DIR/attributes"

zstd -dc $FILE |
    jq -r '.products[] | .attributes | keys[]' |
    sort |
    uniq -c | sort -S1G --parallel=8 -nr > $ATTRIBUTES

chmod a-w $ATTRIBUTES

echo -e "attributes:\t$ATTRIBUTES"

AV_DIR="$DATA_DIR/attribute-values"

mkdir -p "$AV_DIR"

TMP=$(mktemp)

trap "rm $TMP" EXIT

zstd -dc $FILE | jq -r '.products[] | .attributes' > $TMP

cat $ATTRIBUTES |
    awk '{print $2}' |
    parallel -j 1 '
        OUT="'$AV_DIR'/{}"
        cat '"$TMP"' |
        jq -r .{} |
        sort |
        uniq -c |
        sort -rn > "$OUT"
        echo -e "{}:\t$OUT"
    '