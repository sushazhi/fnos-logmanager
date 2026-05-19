#!/bin/bash
# app/ui/router.cgi
# 架构感知 CGI 路由脚本
# x86_64：302 重定向到统一网关
# ARM：反向代理到本地后端端口

TARGET_PORT=8090
GATEWAY_PATH="/app/logmanager"

ARCH=$(uname -m)

if [ "$ARCH" = "x86_64" ]; then
    # x86：302 重定向到统一网关
    echo "Status: 302 Found"
    echo "Location: $GATEWAY_PATH"
    echo ""
    exit 0
fi

# ARM：反向代理到本地后端
URI_PATH="${REQUEST_URI#*router.cgi}"
TARGET_URL="http://127.0.0.1:${TARGET_PORT}${URI_PATH}"
METHOD="$REQUEST_METHOD"

# 构建 curl 参数（数组保证 shell 安全）
CURL_ARGS=(-s --include -X "$METHOD")

# 转发关键请求头
[ -n "$HTTP_COOKIE" ]           && CURL_ARGS+=(-H "Cookie: $HTTP_COOKIE")
[ -n "$HTTP_X_CSRF_TOKEN" ]     && CURL_ARGS+=(-H "X-CSRF-Token: $HTTP_X_CSRF_TOKEN")
[ -n "$HTTP_X_SESSION_TOKEN" ]  && CURL_ARGS+=(-H "X-Session-Token: $HTTP_X_SESSION_TOKEN")
[ -n "$CONTENT_TYPE" ]          && CURL_ARGS+=(-H "Content-Type: $CONTENT_TYPE")
[ -n "$HTTP_ORIGIN" ]           && CURL_ARGS+=(-H "Origin: $HTTP_ORIGIN")

# 根据 HTTP 方法决定是否带请求体
case "$METHOD" in
    POST|PUT|PATCH)
        exec curl "${CURL_ARGS[@]}" --data-binary @- "$TARGET_URL"
        ;;
    *)
        exec curl "${CURL_ARGS[@]}" "$TARGET_URL"
        ;;
esac
