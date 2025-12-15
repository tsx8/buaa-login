# buaa-login

![Build Status](https://img.shields.io/github/actions/workflow/status/tsx8/buaa_login/ci.yml)
![Go Version](https://img.shields.io/badge/Go-1.25.3-blue)

**buaa-login** 是一个用 Go 语言编写的命令行工具，用于自动登录北京航空航天大学（BUAA）校园网网关。

它轻量、跨平台，并专为 NixOS 用户提供了原生支持。

## ✨ 功能特性

*   **跨平台支持**：支持 Windows, Linux (amd64/arm64), macOS。
*   **自动重试**：内置网络抖动处理，登录失败会自动重试 10 次。
*   **SRun 算法支持**：完整实现了校园网认证所需的复杂加密算法（MD5, SHA1, Base64, XEncode）。
*   **NixOS 友好**：提供 Flake 和 NixOS Module，支持开机自动登录服务。

## 🚀 快速开始

### 1. 命令行使用

下载对应系统的二进制文件或自行编译后，使用以下命令登录：

```bash
./buaa-login -i <学号> -p <密码>

./buaa-login -v
```

**示例：**
```bash
./buaa-login -i 23371234 -p MySecretPass
```
如果登录成功，程序会输出 `Login successful!` 并显示账户信息；如果失败，程序会自动重试。

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
  buaa-login.url = "github:tsx8/buaa-login"; # 添加这一行
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
    # 凭据文件路径，文件内容格式为：<学号> <空格> <密码>
    # 例如: 23371234 MyPassword
    configFile = "/etc/nixos/buaa-cred.txt"; 
  };
}
```

*警告：这将导致密码明文存储在世界可读的 Nix Store 中。*

```nix
services.buaa-login = {
  enable = true;
  stuid = "23371234";
  stupwd = "MyPassword";
};
```

### 服务说明
启用后，系统会在 `network-online.target` 达成后自动尝试登录，并在失败时自动重启服务。

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