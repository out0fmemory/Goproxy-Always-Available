配置介绍

## 前言   

　　以当前r804正式版为准。以后版本有出入再修改。这是新人入门用的，不是最详细的配置介绍，以能运行能用为准。最主要的配置文件为 gae.json 和 httpproxy.json。没有写到的地方(选项或者配置文件)一般不要修改，如要修改，前提是你理解它的具体作用或者用途。   
　　注：   
a．当前版本已支持 autorange 和 http2 。默认配置已开启这两项。   
b．当前版本格式已较宽松，即末尾有无逗号都可以。如："ID1","ID2","ID3", 和 "ID1","ID2","ID3" 效果一样。   
c．当前版本支持 xxx.user.json 这种命名格式文件(即，用户配置文件)。升级最新版时可以直接覆盖掉原来的 xxx.json 。   
>方法一：如 gae.json，复制 gae.json 为 gae.user.json 修改并保存。   
>方法二：如 gae.json，使用 [notepad++](http://notepad-plus-plus.org/) 或者 [notepad2](http://www.flos-freeware.ch/notepad2.html) 新建名为 gae.user.json 的文件，文件格式选 UTF-8, 内容举例如下：   
>
>   ```
>   {
>       "AppIDs": [
>           "ID1",
>           "ID2",
>           "ID3",
>       ],
>       "Password": "123456",
>   }
>   ```
>
>即：在新的 gae.user.json 文件里只加上你想要修改的内容(选项)。不要忘了外层大括号"{}"。   

d. 善用搜索。礼貌提问。提问之前最好搜索一下。没有人有问答你问题的义务。别人都很忙，别人时间都是宝贵的。   
e. 不要使用 Windows 系统自带的“记事本”来修改任何配置文件，容易出错(大多数是编码问题)。推荐用上面两个编辑器来修改。   
f. 当前版本已支持双斜杠 "//" 作为注释符号。   

## gae.json 配置文件

GAE代理配置。

* "AppIDs" 选项
>在这里加入你的 Google Appengine 的帐号。格式如下：  
>
>   ```
>   "AppIDs": [
>       "ID1",
>       "ID2",
>       "ID3",
>   ],
>   ```
>
>注：   
>格式使用多行(一行一个AppID)或者一行(不换行)效果一样，但要注意，AppID要使用双引号包含起来，两个AppID之间用逗号(英语输入法下的逗号)分隔，末尾有无逗号都可以。   

* "Password" 选项
> "AppIDs"密码。格式：   
>
>   ```
>   "Password": "密码"
>   ```
>
>注：密码必须和gae服务端里的相同。密码只是用来防止 appid 被别人盗用。 如果服务端没有设密码，这里也不要改动。   
>
>　　服务端密码设置(gae.go文件)   
>
>前提：须先下载 goproxy 服务端 https://github.com/phuslu/goproxy/archive/server.gae.zip   
>
>用文本编辑器打开 gae目录下的 gae.go 文件，不建议使用 Windows 自带的记事本，推荐 [notepad++](http://notepad-plus-plus.org/) 或者 [notepad2](http://www.flos-freeware.ch/notepad2.html)   
>
>找到23行(行数随版本变化不一定对)，如下：   
>
>   ```
>   Version  = "1.0"
>   Password = ""
>   ```
>    
>改成：   
>
>   ```
>   Version  = "1.0"
>   Password = "密码"
>   ```
>    
>保存。重新上传。   

* "SSLVerify" 选项
>作用：验证服务器的SSL证书。检查服务器的SSL证书是否是 google 证书。   
>
>参数：   
>
>   ```
>   false : 关闭。   
>   true : 开启。   
>   ```

* "ForceIPv6" 选项
>作用：强制使用 IPv6 模式。   
>
>参数：   
>
>   ```
>   false : 关闭。   
>   true : 开启。   
>   ```

* "DisableHTTP2" 选项
>作用：关闭 http2 模式，使用 http1 模式。不懂或者不理解 http1 和 http2 的区别不要修改。   
>
>参数：   
>
>   ```
>   false : 默认。先验证IP是否支持http2，否则使用http1。   
>   true : 关闭 http2 模式，所有IP都使用 http1 模式。   
>   ```
>
>此项配合 "HostMap" 选项使用。   

* "ForceHTTP2" 选项
>作用：强制开启 http2 模式。不懂或者不理解 http1 和 http2 的区别不要修改。   
>
>参数：   
>
>   ```
>   false : 默认。   
>   true : 强制开启 http2 模式。所有IP都使用 http2 模式。   
>   ```
>
>此项配合 "HostMap" 选项使用。   

* "EnableDeadProbe" 选项
>作用：是否开启死链接(无效链接)检测   
>
>参数：   
>
>   ```
>   false : 关闭。   
>   true : 开启。   
>   ```

* "EnableRemoteDNS" 选项
>作用：是否启用远程DNS解析。配合 "DNSServers" 选项使用。   
>
>参数：   
>
>   ```
>   false : 关闭。   
>   true : 开启。   
>   ```

* "Sites" 选项
>作用：域名过滤。   
>
>   ```
>   "*" 号表示所有的网址(网站)都走 GAE 代理，即，不做任何过滤。这里一般不用改。
>   ```

* "HostMap" 选项
>这里填写你找到的IP。格式如下：   
>
>   ```
>   "HostMap" : {
>       "google_hk": [
>           "xxx.xxx.xxx.xxx",
>           "xxx.xxx.xxx.xxx",
>           "xxx.xxx.xxx.xxx",
>           "xxx.xxx.xxx.xxx",
>       ],
>       "google_cn": [
>           "xxx.xxx.xxx.xxx",
>           "xxx.xxx.xxx.xxx",
>           "xxx.xxx.xxx.xxx",
>           "xxx.xxx.xxx.xxx",
>       ]
>   },
>   ```
>
>注：   
>a. "google_cn" 子项可以不用修改。默认情况下使用的是 google 中国 IP (服务)。   
>b. 当前版本已支持IP自动去重----即，填入的IP即使有重复，程序会自动去掉重复的再导入使用。   
>c. 格式使用多行(一行一个IP)或者一行(不换行)效果一样，但要注意，IP要使用双引号包含起来，两个IP之间用逗号(英语输入法下的逗号)分隔，末尾有无逗号都可以。   
>d. 此项配合 "SiteToAlias" 选项使用。   

* "SiteToAlias" 选项
>此项配合 "HostMap" 选项使用。   
>
>作用：   
>a.简单说是让哪些谷歌服务(搜索、广告、视频)翻不翻墙(使用谷歌中国IP还是使用谷歌国外IP)。   
>b.添加在这里的表示直连。相应的，不添加在这里的表示走 GAE 代理。   
>
>常用例子：   
>
>如：youtube 出现"上传用户已禁止在您的国家/地区播放此视频" 时取消以下注释。   
>
>   ```
>   // "*.youtube.com": "google_hk",
>   ```
>
>改为：   
>
>   ```
>   "*.youtube.com": "google_hk",
>   ```
>
>注：使用 youtube 直连，可以解决"上传用户已禁止在您的国家/地区播放此视频"问题。但也可能会导致某些视频无法播放，如果是这样，再改回来(注释掉)。

* "ForceGAE" 选项
>作用：哪些网址强制走GAE代理。   

* "ForceDeflate" 选项
>作用：哪些网址强制使用"压缩"功能？   

* "TLSConfig" 选项
>作用：TLS协议配置。   
>
>   * "Version" 子项：TLS协议版本号。   
>
>   * "ClientSessionCacheSize" 子项：   
>
>   * "Ciphers" 子项：   
>
>   * "ServerName" 子项：   

* "GoogleG2KeyID" 选项
>作用：

* "FakeOptions" 选项
>作用：

* "DNSServers" 选项
>作用：远程DNS服务器IP。"EnableRemoteDNS" 选项为 false 时此项无效。   

* "IPBlackList" 选项
>作用：DNS服务器黑名单，此项配合 "DNSServers" 选项使用。"EnableRemoteDNS" 选项为 false 时此项无效。   

* "Transport" 选项
>作用：传输设置。
>
>   * "Dialer" 子项
>   >作用：
>
>   * "Proxy" 子项
>   >作用：GAE 代理的前置代理设置。
>   >
>   >   ```
>   >   "Enabled": 开启或者关闭。参数：true/false
>   >   "URL": 前置代理地址。
>   >   ```
>   >
>
>   * "DisableCompression" 子项
>   >作用：
>
>   * "DisableKeepAlives" 子项
>   >作用：
>
>   * "IdleConnTimeout" 子项
>   >作用：
>
>   * "MaxIdleConnsPerHost" 子项
>   >作用：
>
>   * "ResponseHeaderTimeout" 子项
>   >作用：
>
>   * "RetryDelay" 子项
>   >作用：
>
>   * "RetryTimes" 子项
>   >作用：
>

## httpproxy.json 配置文件

GoProxy 代理设置

>这里分两大部分。 "Default" 和 "PHP" 。这两种代理模式可同时开启。   
>
>   ```
>   "Default" 选项: 即 GAE 代理。此项配合 gae.json 配置文件使用。
>   "PHP" 选项: 即 PHP 代理。此项配合 php.json 配置文件使用。
>   ```
>
>说明：双斜杠 "//" 是注释符号，取消即使用，加上即关闭该功能。   

* "Enabled" 子项
>参数：   
>
>   ```
>   false : 关闭代理("Default" 或者 "PHP")。   
>   true : 开启代理("Default" 或者 "PHP")。   
>   ```

* "Address" 子项
>作用：监听地址和端口。这里一般不用改。   
>
>   格式：
>   ```
>   "IP地址:端口" 
>   ```
>   
>这里常用有两种(任选一种)：   
>
>   ```
>   "Address": "127.0.0.1:8087",
>
>   "Address": "0.0.0.0:8087",
>   ```
>    
>注：   
>1. 第一种是只能本机使用。   
>2. 第二种是多台电脑共享代理时使用。当然本机也能用。   

* "RequestFilters" 子项
>作用：请求过滤。   
>
>一般不要修改。   
>
>   ```
>   "auth": 前置代理？ 此项配合 auth.json 配置文件使用。
>   "rewrite": 自定义UserAgent? 此项配合 rewrite.json 配置文件使用。
>   "stripssl": 使用SSL证书? 此项配合 stripssl.json 配置文件使用。
>   "autorange": 多线程自动传输(下载)？ 此项配合 autorange.json 配置文件使用。
>   ```

* "RoundTripFilters" 子项
>作用：往返过滤。   
>
>一般不要修改。   
>
>   ```
>   "autoproxy": SiteFilters和pac和gfwlist模式？此项配合 autoproxy.json 配置文件使用。
>   "auth": 前置代理。此项配合 auth.json 配置文件使用。
>   "vps": vps代理。此项配合 vps.json 配置文件使用。
>   "php": php代理。此项配合 php.json 配置文件使用。
>   "gae": gae代理。此项配合 gae.json 配置文件使用。
>   "direct": 直连。此项配合 direct.json 配置文件使用。
>   ```

* "ResponseFilters" 子项
>作用：响应过滤。   
>
>一般不要修改。   
>
>   ```
>   "autorange": 多线程自动传输(下载)？ 此项配合 autorange.json 配置文件使用。
>   "ratelimit": 限速？ 此项配合 ratelimit.json 配置文件使用。
>   ```

## php.json 配置文件

PHP 代理配置。   

* "Servers" 选项
>这里写上 PHP 代理地址(网址)和密码。可以同时写上多个代理。   
>
>   ```
>   "Url": PHP代理服务器地址(网址)
>   "Password": PHP代理密码
>   "SSLVerify": ssl证书验证
>   "Host": PHP代理IP地址
>   ```
>
>多个代理格式：   
>
>   ```
>   "Servers": [
>       {
>           "Url": "http://yourapp1.com/",
>           "Password": "123456",
>           "SSLVerify": true,
>           "Host": "xxx.xxx.xxx.xxx",
>       },
>       {
>           "Url": "http://yourapp2.com/",
>           "Password": "123456",
>           "SSLVerify": false,
>           "Host": "",
>       },
>       {
>           "Url": "http://yourapp3.com/",
>           "Password": "123456",
>           "SSLVerify": false,
>           "Host": "",
>       }
>   ],
>   
>   ```

* "Sites" 选项
>作用：域名过滤。   
>
>   ```
>   "*" 号表示所有的网址(网站)都走 PHP 代理，即，不做任何过滤。这里一般不用改。
>   ```
>

* "Transport" 选项
>作用：传输设置。
>
>   * "Dialer" 子项
>   >作用：
>
>   * "Proxy" 子项
>   >作用：PHP 代理的前置代理设置。
>   >
>   >   ```
>   >   "Enabled": 开启或者关闭。参数：true/false
>   >   "URL": 前置代理地址。
>   >   ```
>   >
>
>   * "DisableKeepAlives" 子项
>   >作用：
>
>   * "DisableCompression" 子项
>   >作用：
>
>   * "TLSHandshakeTimeout" 子项
>   >作用：
>
>   * "MaxIdleConnsPerHost" 子项
>   >作用：
>
    
## addto-startup.vbs 脚本文件

>用途：设置 goproxy 开机启动。(windows系统)   

## get-latest-goproxy.cmd 批处理文件

>用途：升级到最新版。(windows系统)   

## auth.json 配置文件

前置代理配置。

* "CacheSize" 选项
>作用：

* "Basic" 选项
>作用：

* "WhiteList" 选项
>作用：

## autoproxy.json 配置文件

自动代理配置。SiteFilters和pac和gfwlist模式？

* "SiteFilters" 选项
>作用：

* "IndexFiles" 选项
>作用：

* "GFWList" 选项
>作用：


## autorange.json 配置文件

文件(特定范围)自动分段传输(下载)。   

特定范围：不提供修改，已写在代码里。范围有：略。

* "Sites" 选项
>作用：域名过滤。即，自定义哪些网址(网站)使用 autorange 功能。   

* "SupportFilters" 选项
>作用：

* "MaxSize" 选项
>作用：

* "BufSize" 选项
>作用：

* "Threads" 选项
>作用：

## direct.json 配置文件

直连配置。   

* "Transport" 选项
> 作用：传输设置。
>
>   * "Dialer" 子项
>   >作用：
>
>   * "TLSClientConfig" 子项
>   >作用：
>
>   * "DisableKeepAlives" 子项
>   >作用：
>
>   * "DisableCompression" 子项
>   >作用：
>
>   * "TLSHandshakeTimeout" 子项
>   >作用：
>
>   * "MaxIdleConnsPerHost" 子项
>   >作用：
>

* (无用项--只作格式化用)

## ratelimit.json 配置文件

限速配置。

* "Threshold" 选项
>作用：

* "Rate" 选项
>作用：

* "Capacity" 选项
>作用：

## rewrite.json 配置文件

* "UserAgent" 选项
>作用：自定义(伪装)用户代理请求头(UserAgent)。   
>
>   * "Enabled" 子项
>
>   >参数：   
>   >
>   >   ```
>   >   false : 关闭。   
>   >   true : 开启。   
>   >   ```
>   >
>   >注：   
>   >开启 "UserAgent" 的步骤具体如下：   
>   >
>   >   ```
>   >   1. httpproxy.json 配置文件 --> "Default" 选项 --> "ResponseFilters" 子项，去掉 "rewrite" 前面的双斜杠 "//" 作为注释符号。
>   >   
>   >   2. 此配置文件里 "Enabled" 子项的参数修改为 "true"。
>   >   ```
>
>   * "Value" 子项
>   >用户代理请求头的具体参数值。不懂或者不理解 http 协议的不要修改。   
>   >
>

* (无用项--只作格式化用)

## stripssl.json 配置文件

* "RootCA" 选项
>作用：

* "Ports" 选项
>作用：

* "Sites" 选项
>作用：

## vps.json 配置文件

VPS 代理配置。   

* "servers" 选项
>服务器端配置。   
>
>   ```
>   "Url": VPS代理地址(网址)
>   "Username": 用户名
>   "Password": 密码
>   "SSLVerify": ssl证书验证
>   ```

* "Sites" 选项
>本地端配置。即，域名过滤。   
>
>   ```
>   "*" 号表示所有的网址(网站)都走 VPS 代理，即，不做任何过滤。这里一般不用改。
>   ```
>
>如需自定义，可以在这修改。格式参考 autorange.json 配置文件 "Sites" 选项。   

