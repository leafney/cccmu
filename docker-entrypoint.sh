#!/bin/sh
set -e

APP_USER=${APP_USER:-appuser}
APP_GROUP=${APP_GROUP:-appuser}
DATA_DIR=${DATA_DIR:-/app/data}

# 输出调试信息
echo "[entrypoint] 启动容器，当前用户: $(id)"
echo "[entrypoint] 数据目录: $DATA_DIR"

# 确保数据目录存在
if [ ! -d "$DATA_DIR" ]; then
  echo "[entrypoint] 创建数据目录: $DATA_DIR"
  mkdir -p "$DATA_DIR"
fi

# 检查当前是否为 root 用户
current_user_id=$(id -u)
if [ "$current_user_id" -eq 0 ]; then
  echo "[entrypoint] 以 root 身份运行，尝试设置权限..."
  
  # 获取目录的当前权限
  dir_uid=$(stat -c '%u' "$DATA_DIR" 2>/dev/null || echo "1000")
  dir_gid=$(stat -c '%g' "$DATA_DIR" 2>/dev/null || echo "1000")
  echo "[entrypoint] 目录当前权限: $dir_uid:$dir_gid"
  
  # 获取目标用户的 UID/GID
  target_uid=$(id -u "$APP_USER" 2>/dev/null || echo "1000")
  target_gid=$(id -g "$APP_GROUP" 2>/dev/null || echo "1000")
  echo "[entrypoint] 目标用户权限: $target_uid:$target_gid"
  
  # 尝试调整目录权限
  if chown "$target_uid:$target_gid" "$DATA_DIR" 2>/dev/null; then
    echo "[entrypoint] ✓ 权限设置成功"
    chmod 755 "$DATA_DIR" 2>/dev/null || true
    
    # 以非 root 用户身份运行
    echo "[entrypoint] 切换到用户 $APP_USER ($target_uid:$target_gid) 运行应用"
    exec su-exec "$APP_USER":"$APP_GROUP" "$@"
  else
    echo "[entrypoint] ⚠️  chown 失败，尝试权限测试..."
    
    # 测试目标用户是否能写入目录
    if su-exec "$APP_USER":"$APP_GROUP" sh -c "touch '$DATA_DIR/.write_test' 2>/dev/null && rm -f '$DATA_DIR/.write_test' 2>/dev/null"; then
      echo "[entrypoint] ✓ 用户 $APP_USER 具有写入权限，以该用户运行"
      exec su-exec "$APP_USER":"$APP_GROUP" "$@"
    else
      echo "[entrypoint] ⚠️  用户 $APP_USER 无写入权限"
      
      # 尝试修改目录权限为通用可写
      if chmod 777 "$DATA_DIR" 2>/dev/null; then
        echo "[entrypoint] ✓ 设置目录为通用可写，以用户 $APP_USER 运行"
        exec su-exec "$APP_USER":"$APP_GROUP" "$@"
      else
        echo "[entrypoint] ⚠️  无法设置目录权限，以 root 身份运行应用"
        echo "[entrypoint] 注意：这可能不是最佳的安全实践"
        exec "$@"
      fi
    fi
  fi
else
  echo "[entrypoint] 非 root 用户运行，检查数据目录权限..."
  
  # 测试当前用户是否能写入目录
  if touch "$DATA_DIR/.write_test" 2>/dev/null && rm -f "$DATA_DIR/.write_test" 2>/dev/null; then
    echo "[entrypoint] ✓ 具有数据目录写入权限"
    exec "$@"
  else
    echo "[entrypoint] ❌ 无数据目录写入权限"
    echo "[entrypoint] 请确保数据目录权限正确或以 root 身份运行容器"
    exit 1
  fi
fi
