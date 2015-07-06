## 常见问题

1. 是否每次更新都要重新上传？
> 更新历史中带有"是"需要重新上传，否则不用重新上传。
> 注意：是否需要重新上传是相对于前一版的，若你之前版本与当前版本之间某一版或多版带有[是]仍然需要重新上传。

1. 遇到 FAQ 没有解决问题怎么办?
> 首先请更新客户端和服务端到最新版(见首页)，如果还有问题的话请提出 issue 。
> 提 issue 前建议先搜索下看是否是重复的问题。虽然我们可能顾不上回答，但是我们保证每个issue都会看的并尝试解决的。

1. 用 goagent 访问twitter，自动跳转为 mobile.twitter.com 并返回 403 Forbidden。
> 检查自己的 id 是不是包含 android 或者 apple, iphone, mobile 啥的。

1. 在 Linux/Mac 下如何安装 gevent?
> easy_install gevent

1. youtube 不能上传以及看直播？
> 请看配置选项 wiki 介绍。https://github.com/goagent/goagent/blob/wiki/ConfigIntroduce.md.ini

1. 提示 HTTP Error code 错误怎么办？
> 400: BAD Request 一般是 iplist 配置不对，尝试使用默认 iplist。
> 401: Unauthorized 一般是你处于内网环境中，需要设置 proxy.ini 里面的 proxy 段落。
> 403: Forbidden 先清空Hosts文件，然后删了proxy.ini [iplist]中的google\_cn=*，并将[profile]的google\_cn改为google\_hk，[iplist]里google\_hk=后面填找到的IP。
> 404: Not Found 一般是 proxy.ini 里面 appid 没有填对，或者服务端没有部署成功。
> 500: 一般是 server/client 版本不匹配，可能是没有上传成功，使用你正在使用的版本重新上传。
> 503: Service Unavailable 一般是流量用完了，请更换appid。

1. GAE 免费流量配额是多少？
> 每个谷歌帐号可以在 GAE 创建10个 appid，每个 appid 每天1G免费流量，urlfetch 每分钟 22M, 传入传出带宽每分钟 56M，GoAgent 使用 urlfetch，故受每分钟 22M 的限制。
> 流量重置时间为[加州时间](http://zh.thetimenow.com/united_states/california/san_francisco)的午夜，夏时制时为北京时间15点，否则为16点。

1. uploader 上传失败？
> 404: Not Found 对应的 appid 没有创建或者 appid 与 Gmail 账户不对应。
> 10060 连接服务器超时，建议挂 VPN 后再上传
> 10054 连接被重置，建议挂 VPN 后再上传
> 10061 目标计算机积极拒绝 挂 VPN 或者运行 goagent 后把IE代理设置为 127.0.0.1:8087
> Cannot set attribute，请暂时停用两部验证，并且到 google.com/settings/security 确认"不够安全的应用的访问权限"已启用。

1. 听说 goagent 保密性比较弱，如何加强？
> 下载最新版的客户端，编辑 proxy.ini, [gae]validate = 1

1. Linux/Mac 如何上传服务端？
> 在 server 目录下运行"python uploader.py"(没有引号)

1. 支持多个 appid 做负载平衡吗？
> 目前 goagent 最新版是支持的，在 proxy.ini 中的配置多个 appid 即可。

1. 如何防止 appid 被别人盗用？
> 请看 <https://goagent.github.io/?/wiki/SetPassword.md> 。

1. 如何使用 php 模式？
> 申请一个免费的 php 空间，然后通过在线代码编辑器或者 ftp 客户端把 index.php 上传到你申请到 php 网站的根目录。
> 假设为 <http://goagent.php.com/index.php> 。访问你的index.php地址，如果没有问题的话，说明部署成功。编辑proxy.ini 
>    ```
>    [php]
>    enable = 1
>    fetchserver = 你的index.php文件的地址
>    ```
> 重启 goagent.exe 即可。


1. 如何设为系统服务（开机自启动）？
> 双击 addto-startup.js 即可。

1. goagent 支持 IPv6 网络吗？
> 支持的，[gae]ipv6 = 1 即可。但是代理后，对网站显示的IP仍是IPv4。

1. 为什么 goagent 第一次运行需要管理员权限？
> 因为 goagent 会尝试向系统导入 IE/Chrome 的证书，这需要管理员权限。

1. Firefox 怎么不能登陆 twitter/facebook 等网站?
> 打开 FireFox->选项->高级->加密->查看证书->证书机构->导入证书, 选择local\CA.crt, 勾选所有项，导入。

1. goagent 原理是什么？
> goagent 是 GAE 应用，原理可以参考这个图 ![代理示意图](https://cloud.githubusercontent.com/assets/195836/4602738/ac950aba-5149-11e4-8976-a2606ba08e05.png)

1. 如何防止 goagent 被匿名使用(盗用)？
> 目前 goagent 最新版是支持的，请见密码设置 wiki 介绍。

1. 怎样设置不显示气泡提示？
> 用 reshacker/exescope 等资源编辑工具把气泡提示字符串清空即可。

1. 如何删除 appengine.google.com 上老的 appid ？
> 可以的，请登录 appengine.google.com 删除。

1. 如何得到 goagent 的源代码？
> goagent 的代码和程序是一起的，源代码就是运行程序。

1. 如何对 goagent 进行修改？
> 客户端代码直接改 local/proxy.py, 改完重启 goagent.exe 即可；服务端改 server/gae.py, 改完用 uploader.bat 上传即可。

1. 为什么要叫 goagent，而不叫 GoProxy？
> 一开始叫 GoProxy 的，后来 Hewig 说软件名字带有 proxy 字样不祥，于是就改成了 goagent。

1. Windows 系统下，出现 ioerror:cannot watch more than 2560 sockets
> 使用 goagent-uv.exe 启动。

1. 为什么使用 goagent 后访问 google.com 仍然跳转到 google.com.hk?
> 你访问Google的IP还是中国的，[profile]里删掉 .google.com= 那行即可，但是搜索时可能会跳出验证码。
> 如果你想用自己 IP 上 Google，但不想被跳转，先访问 https://www.google.com/ncr 一下即可。

1. 出现`Address already in use` 错误。
> 原因：可能是 goagent 已经在运行或者端口被其他软件占用，比如搜狗浏览器开启全网加速会使用 8087 端口，比如旧版 goagent 加入开机启动没有删除、旧版已经在运行。
> 解决办法：关闭旧版 goagent 或者其他占用该端口的软件再重启 goagent 即可。
