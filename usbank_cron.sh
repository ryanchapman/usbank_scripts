#!/bin/bash

TMP=/tmp/$(basename $0).$$
EMAIL="US BANK ERROR <ryan@heatery.com>"

~/bin/usbank.rb 2>&1 >$TMP
if [[ $? != 0 ]]; then
	# error
	cat $TMP | EMAIL="$EMAIL" mutt -s "ERROR: usbank.rb" ryan@heatery.com
	exit 0
fi

(
echo "From: US BANK <ryan@heatery.com>"
echo "To: ryan@heatery.com"
echo "MIME-Version: 1.0"
echo "Content-Type: multipart/mixed;"
echo ' boundary="PAA08673.1018277622/server.domain.com"'
echo "Subject: Transactions"
echo ""
echo "This is a MIME-encapsulated message"
echo ""
echo "--PAA08673.1018277622/server.domain.com"
echo "Content-Type: text/html"
echo ""
cat /tmp/output.html | sed 's/Click on the underlined account name to view your recent transactions\.//g' | \
	sed 's|Available<br>Balance|<B>Available Balance</B>|g'
echo "--PAA08673.1018277622/server.domain.com"
) | sendmail -t

rm -f $TMP
