#!/bin/sh
set -e

APP_USER=${APP_USER:-appuser}
APP_GROUP=${APP_GROUP:-appuser}
DATA_DIR=${DATA_DIR:-/app/data}

# 确保数据目录存在
if [ ! -d "$DATA_DIR" ]; then
  mkdir -p "$DATA_DIR"
fi

# 计算目标 UID/GID（允许通过环境变量覆盖）
if [ -n "$APP_UID" ]; then
  target_uid="$APP_UID"
else
  target_uid=$(stat -c '%u' "$DATA_DIR")
fi

if [ -n "$APP_GID" ]; then
  target_gid="$APP_GID"
else
  target_gid=$(stat -c '%g' "$DATA_DIR")
fi

current_uid=$(id -u "$APP_USER")
current_gid=$(id -g "$APP_USER")

# 对齐组 ID（忽略 root 情况）
if [ "$target_gid" -ne "$current_gid" ] && [ "$target_gid" -ne 0 ]; then
  groupmod -o -g "$target_gid" "$APP_GROUP"
  current_gid="$target_gid"
fi

# 对齐用户 ID（忽略 root 情况）
if [ "$target_uid" -ne "$current_uid" ] && [ "$target_uid" -ne 0 ]; then
  usermod -o -u "$target_uid" "$APP_USER"
  current_uid="$target_uid"
fi

# 更新目录归属，保证容器内可写
chown -R "$APP_USER":"$APP_GROUP" "$DATA_DIR"
chmod 755 "$DATA_DIR"

exec su-exec "$APP_USER":"$APP_GROUP" "$@"
