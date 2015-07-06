## 简易教程

- 部署

  1. 申请 [Google Appengine](https://appengine.google.com) 并创建 appid。
  1. 下载 goagent 最新版 https://github.com/goagent/goagent
  1. 修改 local\proxy.ini 中的 [gae] 下的 appid = 你的appid(多appid请用|隔开)
  1. 运行 uploader.bat 或 uploader.py 开始上传, 成功后即可使用了。

- 使用

  * Windows 用户推荐使用 goagent.exe 托盘图标设置 IE 代理(对其它浏览器也有效)。
  * Chrome/Opera 请安装 [SwitchyOmega](https://github.com/FelisCatus/SwitchyOmega/releases) 插件(下载到本地然后拖放文件到扩展设置)，导入 SwitchyOptions.bak
  * Firefox 请安装 [FoxyProxy](https://addons.mozilla.org/zh-cn/firefox/addon/foxyproxy-standard/) ，Firefox需要导入证书，方法请见 FAQ
  * 出现连接不上的情况可以尝试使用 [gscan](https://github.com/yinqiwen/gscan/) 或 [GoGo Tester](https://github.com/azzvx/gogotester/raw/2.3/GoGo%20Tester/bin/Release/GoGo%20Tester.exe) 测速。
