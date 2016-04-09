## 简易教程

- 部署

  1. 申请 [Google Appengine](https://appengine.google.com) 并创建 appid。
  1. 下载 goproxy 服务端 https://github.com/phuslu/goproxy/archive/server.gae.zip
  1. 检查 谷歌账号 [不够安全的应用的访问权限](https://www.google.com/settings/security/lesssecureapps) 选项，确保处于启用状态。
  1. 运行 uploader.bat 或 uploader.py 开始上传, 成功后即可使用了。

- 使用

  * 下载 goproxy 正式版 https://git.io/goproxy, 复制 gae.json 为 gae.user.json 并填入部署完成的 appid
  * Windows 用户推荐使用 goproxy.exe 托盘图标设置 IE 代理(对其它浏览器也有效)。
  * Chrome/Opera 请安装 [SwitchyOmega](https://github.com/FelisCatus/SwitchyOmega/releases) 插件(下载到本地然后拖放文件到扩展设置)，导入 SwitchyOptions.bak
  * Firefox 请安装 [FoxyProxy](https://addons.mozilla.org/zh-cn/firefox/addon/foxyproxy-standard/) ，Firefox需要导入证书，方法请见 FAQ
  * 出现连接不上的情况可以尝试使用 [checkiptools](https://github.com/xyuanmu/checkiptools) 测速。
