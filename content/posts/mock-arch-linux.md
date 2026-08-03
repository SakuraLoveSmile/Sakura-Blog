---
title: Arch Linux 折腾记：从安装到日常使用
date: 2026-07-05
excerpt: 记录一次完整的 Arch 安装与桌面配置经历，包括踩过的坑和最终留下的配置。
categories: 技术
topic: 系统
tags: [Linux, Arch, 系统]
cover: https://picsum.photos/seed/arch-linux/1280/640
banner: https://picsum.photos/seed/arch-banner/1600/640
poster_headline: 滚动更新与折腾
poster_caption: pacman · yay · systemd · i3
---

## 为什么是 Arch

一句话：**滚动更新 + 极简哲学**。没有预装全家桶，从内核到桌面全部自己装，过程本身就是最好的系统课。

## 安装要点

```bash
# 分区后挂载 chroot
pacstrap /mnt base linux linux-firmware vim
genfstab -U /mnt >> /mnt/etc/fstab
arch-chroot /mnt
```

关键坑位记录：

- **网络**：装完没网是最大坑，`systemctl enable --now dhcpcd` 或 NetworkManager。
- **引导**：EFI 机器记得 `pacman -S efibootmgr` 并正确挂载 `/boot/efi`。
- **中文字体**：`noto-fonts-cjk` 缺了的话，网页全是豆腐块。

## 桌面选择

试过 GNOME 太重、KDE 略花，最后停在 **i3wm**：平铺、键盘驱动、配置即文件。

```text
# ~/.config/i3/config
bindsym $mod+Return exec terminal
bindsym $mod+d exec rofi -show run
bindsym $mod+1 workspace 1
```

> 折腾完你会发现：系统里每一行配置都是自己写的，坏在哪、为什么坏，心里清清楚楚。

## 日常使用

- 包管理：`pacman` + `yay`（AUR）
- 输入法：fcitx5 + rime
- 终端：kitty + zsh + starship

半年滚动更新，炸过两次内核，但每次都能五分钟内定位修复——这就是 Arch 教给你的东西。
