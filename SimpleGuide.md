## 简易教程

- 部署

  1. 申请 [Google Appengine](https://appengine.google.com) 并创建 appid。
  1. 到 https://github.com/phuslu/goproxy-ci/releases/latest 下载 goproxy-gae-rXX.zip 服务端部署文件。
  1. 运行 uploader.bat 或 uploader.py 开始上传, 过程中需访问 google 页面并输入授权码， 上传完成后即可使用了。

- 使用

  * 下载 goproxy 正式版 [https://git.io/goproxy](https://github.com/phuslu/goproxy/releases), 复制 gae.json 为 gae.user.json 并填入部署完成的 appid
  * Windows 用户推荐使用 goproxy-gui.exe 托盘图标设置 IE 代理(对其它浏览器也有效)。
  * Chrome/Opera 请安装 [SwitchyOmega](https://github.com/FelisCatus/SwitchyOmega/releases) 插件(下载到本地然后拖放文件到扩展设置)
  * Firefox 请安装 [FoxyProxy](https://addons.mozilla.org/zh-cn/firefox/addon/foxyproxy-standard/) ，Firefox需要导入证书，方法请见 FAQ
  * 出现连接不上的情况可以尝试使用 [MotherFuckerFang](https://github.com/phuslu/goproxy/issues/654) 测速。
