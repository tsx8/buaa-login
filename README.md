# buaa-login

![Build Status](https://img.shields.io/github/actions/workflow/status/tsx8/buaa-login/ci.yml)
![Go Version](https://img.shields.io/badge/Go-1.25.3-blue)

**buaa-login** 是一个用 Go 语言编写的命令行工具，用于自动登录北京航空航天大学（BUAA）校园网网关。

它轻量、跨平台，并专为 NixOS 用户提供了原生支持。

## ✨ 功能特性

*   **跨平台支持**：支持 Windows amd64、Linux amd64/arm64 和 Apple Silicon macOS。
*   **可配置重试**：支持通过 `-r` 参数自定义重试次数，内置 2 秒重试间隔。
*   **指定网络接口**：Linux 可通过 `--interface` 将网关请求严格绑定到指定接口。
*   **SRun 算法支持**：完整实现了校园网认证所需的复杂加密算法（HMAC-MD5, SHA1, 自定义 Base64, XEncode/TEA）。
*   **NixOS 友好**：提供 Flake 和 NixOS Module，支持开机自动登录、定时检查和唤醒后自动登录。
*   **稳定可靠**：10 秒超时设置，Cookie 持久化，完善的错误处理。

## 🚀 快速开始

### 1. 命令行使用

下载对应系统的二进制文件或自行编译后，使用以下命令登录：

```bash
# 基本登录
./buaa-login -i <学号> -p <密码>

# 登录并在失败时重试 3 次
./buaa-login -i <学号> -p <密码> -r 3

# 从 JSON 凭据文件读取，避免密码出现在进程参数中
./buaa-login --credentials-file /run/secrets/buaa-login.json

# Linux：通过指定网络接口登录
./buaa-login --credentials-file /run/secrets/buaa-login.json --interface wlan0

# 显示版本
./buaa-login -v
```

**参数说明：**
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-i` | 学号 | 必填 |
| `-p` | 密码 | 必填 |
| `--credentials-file` | JSON 凭据文件；不能与 `-i`、`-p` 同时使用 | - |
| `--interface` | Linux 上用于网关请求的网络接口；绑定失败时不回退 | - |
| `-r` | 最大重试次数 | 0 |
| `-v` | 显示版本号 | - |

`-i`/`-p` 适合交互测试，但密码会进入 shell 历史和进程参数。自动运行时应使用权限受限、位于 Nix Store 之外的凭据文件。

退出码可用于服务管理：`0` 表示成功，`1` 表示可重试的网络或网关临时故障，`2` 表示命令行或凭据文件配置错误，`3` 表示认证被拒绝。程序只对退出码 `1` 对应的故障执行 `-r` 重试。

**示例：**
```bash
./buaa-login -i 23371234 -p MySecretPass
```
如果登录成功，程序会输出 `Login successful!`；如果发生临时故障，程序会按 `-r` 指定的次数重试。

### 2. 安装方式

#### 方式 A：下载二进制文件
前往 [Releases](../../releases) 页面下载适合您操作系统的预编译文件。

#### 方式 B：使用 Go 安装
```bash
go install github.com/tsx8/buaa-login/cmd/buaa-login@latest
```

#### 方式 C：Nix Run (临时运行)
```bash
nix run github:tsx8/buaa-login -- -i <学号> -p <密码>
```

---

## ❄️ NixOS 集成指南

本项目提供了完善的 Nix Flake 支持，可以将自动登录配置为系统服务。

### 1. 添加 Input
在你的 `flake.nix` 中添加输入源：

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  buaa-login = {
    url = "github:tsx8/buaa-login";
    inputs.nixpkgs.follows = "nixpkgs";
  };
};
```

### 2. 引入模块
在你的 NixOS 配置中引入模块并启用服务。

```nix
{ config, pkgs, inputs, ... }: {
  imports = [
    inputs.buaa-login.nixosModules.default
  ];

  services.buaa-login = {
    enable = true;
    credentialsFile = "/run/secrets/buaa-login.json";
  };
}
```

凭据文件必须位于 Nix Store 之外，并使用以下 JSON 结构：

```json
{"stuid":"23371234","paswd":"MyPassword"}
```

建议由 sops-nix、agenix 或其他运行时密钥管理工具生成该文件。模块通过 systemd credentials 将文件传给程序。

### 3. 配置选项

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable` | bool | `false` | 是否启用服务 |
| `credentialsFile` | string | 必填 | Nix Store 外的绝对 JSON 凭据文件路径 |
| `retry` | int | `3` | 登录失败最大重试次数 |
| `interval` | string | `null` | 定时检查间隔（如 `"15min"`、`"1h"`），设置后启用定时器模式 |
| `wakeUp` | bool | `true` | 从睡眠/挂起唤醒后自动执行登录检查 |

### 4. 使用示例

#### 基础配置（使用凭据文件）
```nix
services.buaa-login = {
  enable = true;
  credentialsFile = "/run/secrets/buaa-login.json";
};
```

#### 定时检查模式（每 15 分钟检查一次）
```nix
services.buaa-login = {
  enable = true;
  credentialsFile = "/run/secrets/buaa-login.json";
  interval = "15min";  # 每 15 分钟检查一次
  retry = 5;           # 失败时重试 5 次
};
```

#### 禁用唤醒后自动登录
```nix
services.buaa-login = {
  enable = true;
  credentialsFile = "/run/secrets/buaa-login.json";
  wakeUp = false;
};
```

### 服务说明
- **默认模式**（`interval = null`）：服务在 `network-online.target` 达成后自动尝试登录；网络或网关临时故障会每 30 秒持续重试，配置错误和认证被拒绝不会重启服务。
- **定时器模式**（设置 `interval`）：通过 systemd timer 定期触发登录检查，适合网络不稳定的环境。
- **唤醒触发**：默认在从睡眠/挂起唤醒后自动执行登录检查。

---

## 🛠️ 从源码构建

如果您想自行修改或构建项目：

1.  **环境要求**：Go 1.25.3
2.  **克隆仓库**：
    ```bash
    git clone https://github.com/tsx8/buaa-login.git
    cd buaa-login
    ```
3.  **编译**：
    ```bash
    go build -ldflags="-s -w" -o buaa-login ./cmd/buaa-login
    ```

---

## 📄 免责声明

本工具仅供学习交流使用。请妥善保管您的校园网账号密码。开发者不对因使用本工具导致的任何账号安全问题或网络滥用行为负责。

## 📜 许可证

MIT License
