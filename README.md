# Fuck-YuKeTang

## 一、介绍

**Fuck-YuKeTang 是一个一堆屎山代码堆砌而成的雨课堂自动答题程序，目前项目尚在完善中，欢迎各位大佬提供意见和建议。**

## 二、功能实现

- [X] 微信扫描二维码登录雨课堂
- [X] 第二次免扫码登录
- [X] 一键签到
- [X] 自动提交问题答案（百分百正确if your net works well）
- [X] 保存PPT
- [X] 收集题库
- [X] 保存运行日志
- [ ] 随机秒数之后提交答案
- [ ] 全部答案第一时间发送至邮箱
- [ ] 点名通知发送至邮箱
- [ ] 课堂答题结果总结发送至邮箱
- [ ] 更多功能等待你的建议......

## 三、使用截图

![1726672468114](image/README/1726672468114.png)

## 四、使用教程

 **1.准备好一个能上网的windows系统的电脑 （孩子们，这个做不到这辈子就有了(bushi)）**

 **2.下载程序(见下方下载链接)，并双击运行**

 **3.微信扫描二维码登录雨课堂，并点击允许**

 **4.第一次运行会在程序同级目录下生成一个名为courseId.txt的文件，根据你想要上的课程，将ID复制粘贴到config.yml文件的courseId中**

 **5.再次双击运行程序即可**

## 五、文件结构

```
Fuck-YuKeTang
|-- bank.json       (第一次运行程序生成--题库存储位置)
|-- config.yml      (第一次运行程序生成--默认配置)
|-- courseId.txt    (第一次成功登录后生成--当前账号的所有课程对应的ID表，填入config.yml文件中courseId)
|-- log.txt         (第一次运行程序生成--日志存储位置)
|-- Fuck-YuKeTang   (可执行程序)
|
|______ppts         (第一次运行程序生成--保存的PPT位置)
```

## 七、下载（建议小白直接使用第一个）

**1. [Windows(amd64)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_windows_amd64.exe)**

**2. [Windows(X86)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_windows_386.exe)**

**3. [Windows(arm64)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_windows_arm64.exe)**

**4. [Windows(arm)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_windows_arm.exe)**

**5. [Linux(X86)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_linux_386)**

**6. [Linux(amd64)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_linux_amd64)**

**7. [Linux(arm)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_linux_arm)**

**8. [Linux(arm64)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_linux_arm64)**

**9. [MacOS(amd64)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_darwin_amd64)**

**10. [MacOS(arm64)](https://ghproxy.mioe.me/https://github.com/only9464/Fuck-YuKeTang/releases/download/v0.0.2/Fuck-YuKeTang_darwin_arm64)**

**11. [更多版本......](https://github.com/only9464/Fuck-YuKeTang/releases)**

## 八、注意事项

**1. 本程序仅供学习交流使用，请勿用于商业用途，限24小时内删除，否则后果自负**

**2. 公测期间，软件免费，请勿用于商业用途，否则后果自负**

**3. 程序运行期间关闭此程序将不再自动答题**

**4. 如有同志想要一同完善该屎山项目，请发送邮件至sky9464@qq.com[联系作者](mailto:sky9464@qq.com)**

## 九、更新日志

**2024/09/18**

- 发布windows下的x86架构程序和amd64架构程序
- 实现微信扫描二维码登录雨课堂功能
- 实现保存PPT功能
- 实现保存运行日志功能
- 实现自动提交问题答案功能
