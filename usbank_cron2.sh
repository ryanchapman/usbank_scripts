#!/bin/bash

TMP=/tmp/$(basename $0).$$
EMAIL="US BANK ERROR <ryan@rchapman.org>"

./usbank -outputFile /tmp/output.html 2>&1 >$TMP
if [[ $? != 0 ]]; then
	# error
   (
    cat <<vEOF
From: US BANK <ryan@rchapman.org>
To: ryan@rchapman.org
MIME-Version: 1.0
Content-Type: multipart/mixed;
 boundary="PAA08673.1018277622/server.domain.com"
Subject: ERROR

This is a MIME-encapsulated message

--PAA08673.1018277622/server.domain.com
Content-Type: text/html

vEOF
cat $TMP
echo "--PAA08673.1018277622/server.domain.com"
    ) | /usr/sbin/sendmail -t
	exit 0
fi

(
echo "From: US BANK <ryan@rchapman.org>"
echo "To: ryan@rchapman.org"
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
