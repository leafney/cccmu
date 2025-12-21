# CCCMU Supervisor 部署指南

## 1. 系统准备

### 创建系统用户
```bash
sudo useradd -r -d /opt/cccmu -s /bin/false cccmu
```

### 创建目录结构
```bash
sudo mkdir -p /opt/cccmu/{bin,data}
sudo mkdir -p /var/log/cccmu
```

## 2. 部署应用

### 复制二进制文件
```bash
# 先构建应用
make build

# 复制到部署目录
sudo cp ./bin/cccmu /opt/cccmu/bin/
sudo chmod +x /opt/cccmu/bin/cccmu
```

### 设置权限
```bash
sudo chown -R cccmu:cccmu /opt/cccmu
sudo chown -R cccmu:cccmu /var/log/cccmu
```

## 3. 配置 Supervisor

### 复制配置文件
```bash
sudo cp ./deploy/supervisor.conf /etc/supervisor/conf.d/cccmu.conf
```

### 重新加载配置
```bash
sudo supervisorctl reread
sudo supervisorctl update
```

## 4. 服务管理命令

### 基本控制
```bash
# 启动服务
sudo supervisorctl start cccmu

# 停止服务
sudo supervisorctl stop cccmu

# 重启服务
sudo supervisorctl restart cccmu

# 查看状态
sudo supervisorctl status cccmu
```

### 日志管理
```bash
# 查看实时日志
sudo tail -f /var/log/cccmu/cccmu.log

# 查看服务状态日志
sudo supervisorctl tail cccmu

# 查看服务错误日志
sudo supervisorctl tail cccmu stderr
```

### 配置重载
```bash
# 修改配置后重新加载
sudo supervisorctl reread
sudo supervisorctl update

# 重启整个 supervisor
sudo systemctl restart supervisor
```

## 5. 访问服务

服务启动后可通过以下地址访问：
- http://localhost:8080

## 6. 故障排除

### 检查服务状态
```bash
sudo supervisorctl status cccmu
```

### 查看详细日志
```bash
sudo tail -100 /var/log/cccmu/cccmu.log
```

### 手动测试服务
```bash
sudo -u cccmu /opt/cccmu/bin/cccmu -p 8080 -l
```

### 检查端口占用
```bash
sudo netstat -tlnp | grep 8080
```