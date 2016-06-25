配置介绍

## 前言   

　　以当前r758正式版为准。以后版本有出入再修改。这是新人入门用的，不是最详细的配置介绍，以能运行能用为准。最主要的配置文件为 gae.json 和 httpproxy.json。没有写到的地方(文件)一般不要修改，如要修改，前提是你理解它的具体作用或用途。   
　　注：   
a．当前版本已支持 autorange 和 http2 。默认配置已开启这两项。   
b．当前版本格式已较宽松，即末尾有无逗号都可以。如："ID1","ID2","ID3", 和 "ID1","ID2","ID3" 效果一样。   
c．当前版本支持 xxx.user.json 这种命名格式文件(即，用户配置文件)。升级最新版时可以直接覆盖 xxx.json 。   
方法一：如 gae.json，复制 gae.json 为 gae.user.json 修改并保存。  
方法二：如 gae.json，使用 [notepad++](http://notepad-plus-plus.org/) 或者 [notepad2](http://www.flos-freeware.ch/notepad2.html) 新建名为 gae.user.json 的文件，文件格式选 UTF-8, 内容举例如下：   

```
{
    "AppIDs": [
        "ID1",
        "ID2",
        "ID3",
    ],
    "Password": "123456",
}
```

即：在新的 gae.user.json 文件里只加上你想要修改的内容(选项)。不要忘了外层大括号"{}"。   

d. 善用搜索。礼貌提问。提问之前最好搜索一下。没有人有问答你问题的义务。别人都很忙，别人时间都是宝贵的。   
e. 不要使用 Windows 系统自带的“记事本”来修改任何配置文件，容易出错(大多数是编码问题)。推荐用上面两个编辑器来修改。   

## gae.json 文件

* "AppIDs" 选项

　　在这里加入你的 Google Appengine 的帐号。格式如下：  

```
"AppIDs": [  
    "ID1" ,   
    "ID2" ,    
    "ID3" , 
],   
```
    
* "Password" 选项   

格式："Password": "密码写在这里"   

注：密码必须和gae服务端里的相同。密码只是用来防止 appid 被别人盗用。 如果服务端没有设密码，这里也不要改动。     

　　服务端密码设置(gae.go文件)   

前提：须先下载 goproxy 服务端 https://github.com/phuslu/goproxy/archive/server.gae.zip   

用文本编辑器打开 gae目录下的 gae.go 文件，不建议使用 Windows 自带的记事本，推荐 [notepad++](http://notepad-plus-plus.org/) 或者 [notepad2](http://www.flos-freeware.ch/notepad2.html)   

找到23行(行数随版本变化不一定对)，如下：   

```
Version  = "1.0"
Password = ""
```
    
改成：   

```
Version  = "1.0"
Password = "你的密码"
```
    
保存。重新上传。

* "SSLVerify" 选项

作用：验证服务器的SSL证书。检查服务器的SSL证书是否是 google 证书。

```
false : 关闭。   
true : 开启。   
```

* "ForceIPv6" 选项

作用：强制使用 IPv6 模式。

```
false : 关闭。   
true : 开启。   
```

* "DisableHTTP2" 选项

作用：关闭 http2 模式，使用 http1 模式。不懂或不理解 http1 和 http2 的区别不要修改。   

参数：   

```
false : 默认。先验证IP是否支持http2，否则使用http1。   
true : 关闭 http2 模式，所有IP使用http1。   
```

* "ForceHTTP2" 选项

作用：强制开启 http2 模式。不懂或不理解 http1 和 http2 的区别不要修改。   

参数：   

```
false : 默认。   
true : 强制开启 http2 模式。所有IP使用http2。   
```
    
* "EnableDeadProbe" 选项

作用：是否开启死链接(无效链接)检测

```
false : 关闭。   
true : 开启。   
```

* "EnableRemoteDNS" 选项

作用：是否启用远程DNS解析。配合 "DNSServers" 选项使用。

```
false : 关闭。   
true : 开启。   
```

* "HostMap" 选项

这里填写你找到的IP。格式如下：

```
"HostMap" : {
    "google_hk": [
        "xxx.xxx.xxx.xxx",
        "xxx.xxx.xxx.xxx",
        "xxx.xxx.xxx.xxx",
        "xxx.xxx.xxx.xxx",
    ],
    "google_cn": [
        "xxx.xxx.xxx.xxx",
        "xxx.xxx.xxx.xxx",
        "xxx.xxx.xxx.xxx",
        "xxx.xxx.xxx.xxx",
    ]
},
```

注：   
a. "google_cn" 子项可以不用修改。默认情况下使用的是 google 中国 IP (服务)。   
b. 当前版本已支持IP自动去重----即，填入的IP即使有重复，程序会自动去掉重复的再导入使用。   
c. 此项配合 "SiteToAlias" 选项使用。   

* "SiteToAlias" 选项

此项配合 "HostMap" 选项使用。   

作用：简单说是让哪些谷歌服务(搜索、广告、视频)翻不翻墙(使用谷歌中国IP还是使用谷歌外国IP)。


* "ForceGAE" 选项

作用：哪些网址强制走GAE代理。

* "ForceDeflate" 选项

作用：哪些网址强制使用"压缩"功能？


* "TLSConfig" 选项

作用：TLS协议配置。   

"Version" 子项：TLS协议版本号。   

"ClientSessionCacheSize" 子项：   

"Ciphers" 子项：   

"ServerName" 子项：   



* "GoogleG2KeyID" 选项


* "FakeOptions" 选项


* "DNSServers" 选项

作用：远程DNS服务器IP。"EnableRemoteDNS" 选项为 false 时此项无效。   


* "IPBlackList" 选项

作用：DNS服务器黑名单，此项配合 "DNSServers" 选项使用。"EnableRemoteDNS" 选项为 false 时此项无效。

* "Transport" 选项


    
## httpproxy.json 文件

这里分两大部分。 "Default" 和 "PHP" 。这两种代理模式可同时开启。   

"Default" : 即 GAE 代理。   
"PHP" : 即 PHP 代理。   

说明：双斜杠 "//" 是注释符号，取消即使用，加上即关闭该功能。   

* "Enabled" 选项

参数：   

```
false : 关闭代理("Default" 或者 "PHP")。   
true : 开启代理("Default" 或者 "PHP")。   
```

* "Address" 选项

这里一般不用改。格式： "IP地址:端口"

这里常用有两种(任选一种)：

```
"Address": "127.0.0.1:8087",

"Address": "0.0.0.0:8087",
```
    
注：   
1. 第一种是只能本机使用。   
2. 第二种是用来共享时使用。当然本机也能用。

* "RequestFilters" 选项

作用：请求过滤。

一般不要修改。   

"auth": 前置代理？   
"rewrite": 使用自定义UserAgent?   
"stripssl": 使用SSL证书?   
"autorange": 多线程传输(下载)？   

* "RoundTripFilters" 选项

作用：往返过滤。

一般不要修改。   

"autoproxy": SiteFilters或pac或gfwlist模式？   
"auth": 前置代理   
"vps": vps代理   
"php": php代理   
"gae": gae代理   
"direct": 直连   


* "ResponseFilters" 选项

作用：响应过滤。

一般不要修改。   

"autorange": 多线程传输(下载)？   
"ratelimit": 限速？   

## php.json 文件

PHP 代理配置。   

* "Servers" 选项

这里写上 PHP 代理地址(网址)和密码。可以同时写上多个代理。   

"Url": PHP 代理地址(网址)   
"Password": PHP 代理密码   
"SSLVerify": 开启ssl证书验证   
"Host": PHP 代理IP地址   

多个代理格式：   

```
"Servers": [
    {
        "Url": "http://yourapp1.com/",
        "Password": "123456",
        "SSLVerify": true,
        "Host": "xxx.xxx.xxx.xxx",
    },
    {
        "Url": "http://yourapp2.com/",
        "Password": "123456",
        "SSLVerify": false,
        "Host": "",
    },
    {
        "Url": "http://yourapp3.com/",
        "Password": "123456",
        "SSLVerify": false,
        "Host": "",
    }
],
    
```
    
## addto-startup.vbs 文件

用途：设置 goproxy 开机启动。(windows系统)   


## get-latest-goproxy.cmd 文件

用途：升级到最新版。(windows系统)   

## auth.json 文件


## autoproxy.json 文件

* "SiteFilters" 选项

* "IndexFiles" 选项

* "GFWList" 选项


## autorange.json 文件

* "Sites" 选项

* "SupportFilters" 选项


* "MaxSize" 选项

* "BufSize" 选项


* "Threads" 选项


## direct.json 文件


## ratelimit.json 文件


## rewrite.json 文件

* "UserAgent" 选项



## stripssl.json 文件


## vps.json 文件


