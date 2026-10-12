#!/bin/sh
# Serves the sidebar launcher for the ingress root. busybox httpd runs this
# for "/" because there is no index.html, which lets it send no-cache
# headers: otherwise phones keep an old launcher after an add-on update.
printf 'Content-Type: text/html; charset=utf-8\r\nCache-Control: no-store\r\n\r\n'
cat /var/www/launcher.html
