配置介绍(非官方)

## 前言   

　　以r1058版为准。以后版本有出入再修改。这是新人入门用的，不是最详细的配置介绍，以能运行能用为准。最主要的配置文件为 httpproxy.json 和 gae.json 。没有写到的地方(选项或者配置文件)一般不要修改，如要修改，前提是你理解它的具体作用或者用途。   
　　注：   
a．首次运行 GoProxy ，Windows 系统建议用 "右键" --> "以管理员身份运行" ，以解决证书问题(自动导入证书)。   
b．当前版本已支持 autorange 和 HTTP/2 。默认配置已开启这两项(有时默认配置会关闭这两项，具体以实际情况为准)。   
c．当前版本格式已较宽松，即末尾有无逗号都可以。如："ID1","ID2","ID3", 和 "ID1","ID2","ID3" 效果一样。   
d．当前版本支持 xxx.user.json 这种命名格式文件(即，用户配置文件)。升级最新版时可以直接覆盖掉原来的 xxx.json 。   
>方法一：如 gae.json，把 gae.user.json.example 改为 gae.user.json ，添加修改并保存。   
>
>方法二：如 gae.json，复制 gae.json 为 gae.user.json 修改并保存。   
>
>方法三：如 gae.json，使用 [notepad++](http://notepad-plus-plus.org/) 或者 [notepad2](http://www.flos-freeware.ch/notepad2.html) 新建名为 gae.user.json 的文件，文件格式选 "UTF-8", 内容举例如下：   
>
>	```
>	{
>		"AppIDs": [
>			"AppID1",
>			"AppID2",
>			"AppID3",
>			],
>		"Password": "123456",
>	}
>	```
>
>即：在新的 gae.user.json 文件里只须加上你想要修改的内容(选项)。不要忘了外层大括号"{}"。   
>
>补充：   
>这里的 UTF-8 编码 在不同编辑器有不同的叫法，所以要以你所用的编辑器为准。具体Google 。   
>notepad++ 中是 "UTF-8 无BOM格式" (UTF-8 without BOM)。   
>notepad2 中是 "UTF-8" 。   
>EditPlus 中是 "UTF-8" 。   
>

e．善用搜索。礼貌提问。提问之前最好搜索一下。《提问的原则》《提问的智慧》，把问题表达清楚。   
f．不要使用 Windows 系统自带的 "记事本" 来修改任何配置文件，容易出错(大多数是编码问题)。推荐用上面两个编辑器来修改。   
g．当前版本已支持双斜杠 "//" 作为注释符号。去掉注释符号即使用，加上注释符号即关闭该功能。   
h．有很多功能的开启都是跨越几个配置文件，且有好多个参数影响。切记。   

## httpproxy.json 配置文件

GoProxy 代理设置   

>httpproxy.json 配置文件是 GoProxy 的全局设置文件。   
>这里分两大部分。 "Default" 和 "PHP" 。这两种代理模式可同时开启。   
>
>	```
>	"Default" 选项: 即 GAE 代理设置。此项配合 gae.json 配置文件使用。
>	"PHP" 选项: 即 PHP 代理设置。此项配合 php.json 配置文件使用。
>	```
>

* "Enabled" 子项
>参数：   
>
>	```
>	false : 关闭代理("Default" 或者 "PHP")。   
>	true : 开启代理("Default" 或者 "PHP")。   
>	```

* "Address" 子项
>作用：监听地址和端口。这里一般不用改。   
>
>	格式：   
>	```
>	"IP地址:端口" 
>	```
>   
>这里常用有两种(任选一种)：   
>
>	```
>	"Address": "127.0.0.1:8087",
>	
>	"Address": "0.0.0.0:8087",
>	```
>    
>注：   
>a．第一种是只能本机使用。   
>b．第二种是多台电脑共享 GoProxy 代理时使用。本机也能使用。   

* "KeepAlivePeriod" 子项
>作用：设置持久连接保持的时间(时长)。
>(httpproxy/httpproxy.go)
>

* "ReadTimeout" 子项
>作用：(HTTP连接)请求的读取操作的超时时间。(服务器超时设置)
>

* "WriteTimeout" 子项
>作用：(HTTP连接)响应的写入操作的超时时间。(服务器超时设置)
>

* "RequestFilters" 子项
>作用：(连接)请求筛选。   
>
>支持以下类型和方法。不懂或者不理解的不要修改。   
>
>	```
>	"auth": 认证。此项配合 auth.json 配置文件使用。要使用此功能就必须先开启，下同。
>	"rewrite": 重写(伪装UserAgent等)。此项配合 rewrite.json 配置文件使用。
>	"autoproxy": 自动代理。此项配合 autoproxy.json 配置文件使用。
>	"stripssl": 伪造与替换SSL证书。 此项配合 stripssl.json 配置文件使用。
>	"autorange": 自动分段传输(下载)。 此项配合 autorange.json 配置文件使用。
>	```
>

* "RoundTripFilters" 子项
>作用：(连接)往返筛选。   
>
>支持以下类型和方法。不懂或者不理解的不要修改。   
>
>	```
>	"autoproxy": 自动代理。此项配合 autoproxy.json 配置文件使用。要使用此功能就必须先开启，下同。
>	"auth": 认证。此项配合 auth.json 配置文件使用。
>	"vps": vps代理。此项配合 vps.json 配置文件使用。
>	"php": php代理。此项配合 php.json 配置文件使用。
>	"gae": gae代理。此项配合 gae.json 配置文件使用。
>	"direct": 直连。此项配合 direct.json 配置文件使用。
>	```
>

* "ResponseFilters" 子项
>作用：(连接)响应筛选。   
>
>支持以下类型和方法。不懂或者不理解的不要修改。   
>
>	```
>	"autorange": 自动分段传输(下载)。 此项配合 autorange.json 配置文件使用。要使用此功能就必须先开启，下同。
>	"rewrite": 重写。此项配合 rewrite.json 配置文件使用。
>	"ratelimit": 限速。 此项配合 ratelimit.json 配置文件使用。
>	```
>

## gae.json 配置文件

GAE代理配置。   

* "AppIDs" 选项
>在这里添加你的 Google App Engine 帐号。格式如下：   
>
>	```
>	"AppIDs": [
>		"AppID1",
>		"AppID2",
>		"AppID3",
>	],
>	```
>
>注：   
>格式使用多行(一行一个AppID)或者一行(不换行)效果一样，但要注意，AppID要使用双引号包含起来，两个AppID之间用逗号(英语输入法下的逗号)分隔，末尾有无逗号都可以。   

* "Password" 选项
> "AppIDs" 帐号对应的密码。格式：   
>
>	```
>	"Password": "密码"
>	```
>
>此项配合 "AppIDs" 选项使用。   
>   
>注：密码必须和 gae 服务端里的相同。密码只是用来防止 AppID 被别人盗用。 如果服务端没有设密码，这里也不要改动。   
>
>　　服务端密码设置(gae.go文件)   
>
>前提：须先下载 gae 服务端 https://github.com/phuslu/goproxy/archive/server.gae.zip 。   
>
>用文本编辑器打开 gae目录下的 gae.go 文件，不建议使用 Windows 自带的记事本，推荐 [notepad++](http://notepad-plus-plus.org/) 或者 [notepad2](http://www.flos-freeware.ch/notepad2.html) 。   
>
>找到23行(行数随版本变化不一定对)，如下：   
>
>	```
>	Version  = "1.0"
>	Password = ""
>	```
>    
>改成：   
>
>	```
>	Version  = "1.0"
>	Password = "密码"
>	```
>    
>保存。重新上传。   

* "SSLVerify" 选项
>作用：验证服务器的SSL证书。检查服务器的SSL证书是否是 Google 证书。   
>[Google 证书 https://pki.google.com/GIAG2.crt](https://pki.google.com/GIAG2.crt)   
>此项配合 "GoogleG2KeyID" 选项使用。   
>
>参数：   
>
>	```
>	false : 关闭。   
>	true : 开启。   
>	```
>

* "DisableIPv6" 选项
>作用：是否关闭 IPv6 模式。   
>此项配合 "ForceIPv6" 选项使用。
>
>参数：   
>
>	```
>	false : 开启 IPv6 模式。   
>	true : 关闭 IPv6 模式。   
>	```
>

* "ForceIPv6" 选项
>作用：强制使用 IPv6 模式。   
>此项配合 "DisableIPv6" 选项使用。
>
>参数：   
>
>	```
>	false : 关闭。   
>	true : 开启。   
>	```

* "DisableHTTP2" 选项
>作用：是否关闭 HTTP/2 模式，使用 HTTP/1.1 模式。不懂或者不理解 HTTP/1.1 和 HTTP/2 的区别不要修改。   
>
>参数：   
>
>	```
>	false : 开启 HTTP/2 模式。先验证IP是否支持 HTTP/2 ，否则使用 HTTP/1.1 。   
>	true : 关闭 HTTP/2 模式，所有IP都使用 HTTP/1.1 模式。   
>	```
>
>此项配合 "HostMap" 选项使用。   

* "ForceHTTP2" 选项
>作用：强制开启 HTTP/2 模式。不懂或者不理解 HTTP/1.1 和 HTTP/2 的区别不要修改。   
>
>参数：   
>
>	```
>	false : 默认。   
>	true : 强制开启 HTTP/2 模式。所有IP都使用 HTTP/2 模式。   
>	```
>
>注：   
>a．此项开启时，不能使用 GAE 和 PHP 的前置代理。   
>b．此项配合 "HostMap" 选项使用。   

* "EnableDeadProbe" 选项
>作用：是否开启死链接(无效链接)检测。   
>
>参数：   
>
>	```
>	false : 关闭。   
>	true : 开启。   
>	```
>
>注:   
>a．使用 GAE 和 PHP 前置代理时建议关闭此项。   
>

* "EnableRemoteDNS" 选项
>作用：是否启用远程DNS解析。   
>此项配合 "DNSServers" 选项使用。   
>
>参数：   
>
>	```
>	false : 关闭。   
>	true : 开启。   
>	```

* "HostMap" 选项
>hosts映射。即，把所有的 "Google" 域名映射到下面的IP地址(或者是DNS解析得到的IP地址)。   
>这里填写你找到的IP。对多IP有很好的支持，推荐填入N多个IP。格式如下：   
>
>	```
>	"HostMap" : {
>		"google_hk": [
>			"xxx.xxx.xxx.xxx",
>			"xxx.xxx.xxx.xxx",
>			"xxx.xxx.xxx.xxx",
>			"xxx.xxx.xxx.xxx",
>		],
>		"google_cn": [
>			"xxx.xxx.xxx.xxx",
>			"xxx.xxx.xxx.xxx",
>			"xxx.xxx.xxx.xxx",
>			"xxx.xxx.xxx.xxx",
>		]
>	},
>	```
>
>注：   
>a．"google_cn" 子项可以不用修改。默认情况下使用的是 Google 中国 IP (服务)。   
>b．当前版本已支持IP自动去重----即，填入的IP即使有重复，程序会自动去掉重复的再导入使用。   
>c．格式使用多行(一行一个IP)或者一行(不换行)效果一样，但要注意，IP要使用双引号包含起来，两个IP之间用逗号(英语输入法下的逗号)分隔，末尾有无逗号都可以。   
>d．此项配合 "SiteToAlias" 选项使用。   

* "SiteToAlias" 选项
>"Google" 网站别名。即，"Google" 域名映射到("HostMap" 里的)哪一个上面("google_hk" 或者 "google_cn")。   
>支持通配符 "*" 。   
>此项配合 "HostMap" 选项使用。   
>
>作用：   
>a．简单说是让哪些谷歌服务(搜索、广告、视频)翻不翻墙(使用谷歌中国IP还是使用谷歌国外IP)。   
>b．添加在这里的表示直连("HostMap")。相应的，不添加在这里的表示走 GAE 代理。   
>
>常用例子：   
>
>如：youtube 出现"上传用户已禁止在您的国家/地区播放此视频" 时取消以下注释。   
>
>	```
>	// "*.youtube.com": "google_hk",
>	```
>
>改为：   
>
>	```
>	"*.youtube.com": "google_hk",
>	```
>
>注：使用 youtube 直连，可以解决"上传用户已禁止在您的国家/地区播放此视频"问题。但也可能会导致某些视频无法播放，如果是这样，再改回来(注释掉)。

* "ForceGAE" 选项
>作用：哪些网址强制走GAE代理。只支持 "Google" 域名。   
>一些 "SiteToAlias" 里的 "Google" 网站直连 "HostMap" IP会出现问题，所以让其强制走 GAE 代理来排除。   
>此项配合 "SiteToAlias" 和 "HostMap" 选项使用。   
>

* "TLSConfig" 选项
>作用：TLS协议配置。   
>
>	* "Version" 子项：
>	>作用：TLS协议版本号。   
>
>	* "ClientSessionCacheSize" 子项：   
>	>作用：TLS会话缓存大小。   
>
>	* "Ciphers" 子项：   
>	>作用：ssl cipher。对称加密算法和不对称加密算法的组合。   
>
>	* "ServerName" 子项：   
>	>作用：TLS Server Name Indication (SNI)。伪装 ServerName。   
>

* "GoogleG2KeyID" 选项
>作用：Google 证书。(Base64编码)   
>此项配合 "SSLVerify" 选项使用。   

* "FakeOptions" 选项
>作用：自定义 HTTP OPTIONS 请求头。   

* "DNSServers" 选项
>作用：远程 DNS 服务器 IP 。   
>"EnableRemoteDNS" 选项为 false 时此项无效。   
>例外：在 Windows 系统中，如果在 "本地连接" 中已经自定义了 DNS 服务器，也必须在这里添加。否则自定义 DNS 服务器在 GoProxy 中无效。   
>
>例子：   
>如：在 Windows 系统中，已在 "本地连接" 中自定义 DNS 服务器为 127.0.0.1 ，则选项必须修改为：   
>
>	```
>	"DNSServers": [
>		"127.0.0.1",
>	],
>	```
>

* "IPBlackList" 选项
>作用：DNS 服务器黑名单。   
>此项配合 "DNSServers" 选项使用。   

* "Transport" 选项
>作用: 传输设置。   
>
>	* "Dialer" 子项
>	>作用：连接配置。   
>	>
>	>	```
>	>	"DNSCacheExpiry": DNS缓存时间(TTL)。
>	>	"DNSCacheSize": DNS缓存大小。
>	>	"SocketReadBuffer": 套接字读取缓冲区大小。
>	>	"DualStack": 双栈(IPv4和IPv6)。
>	>	"KeepAlive": 持久连接保持的时间(时长)。
>	>	"Level": IP筛选并发数(层次/级别)。(r1041 httpproxy/dialer/dialer.go httpproxy/dialer/dialer2.go)
>	>	"Timeout": 建立连接超时。
>	>	```
>	>
>
>	* "Proxy" 子项
>	>作用: GAE 代理的前置代理设置。前置代理支持 http、ssh、socks4 和 socks5 。   
>	>
>	>	```
>	>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	>	```
>	>	```
>	>	"URL": 前置代理地址。
>	>	
>	>	具体格式如下：
>	>	
>	>	http://XXX.XXX.XXX.XXX:1080         http1代理(http代理)
>	>	http1://XXX.XXX.XXX.XXX:1080         http1代理(http代理)
>	>	socks4://XXX.XXX.XXX.XXX:1080       socks4代理
>	>	socks4a://XXX.XXX.XXX.XXX:1080       socks4a代理
>	>	socks://XXX.XXX.XXX.XXX:1080       socks5代理
>	>	socks5://XXX.XXX.XXX.XXX:1080       socks5代理
>	>	ssh://XXX.XXX.XXX.XXX:1080       ssh代理(此项配合 ssh2.json 配置文件使用)
>	>	ssh://XXX.XXX.XXX.XXX:1080       ssh代理(此项配合 ssh2.json 配置文件使用)
>	>	```
>	>
>	> (r833 httpproxy/proxy/proxy.go#L86-L90)   
>	> (r932 httpproxy/filters/gae/gae.go#L249-L264)   
>	>
>	>使用前置代理时，关闭以下选项：   
>	>a．关闭 "ForceHTTP2" 选项。   
>	>b．关闭 "EnableDeadProbe" 选项。   
>	>
>
>	* "DisableCompression" 子项
>	>作用: 是否关闭压缩。(Accept-Encoding/Accept: Gzip)   
>	>
>	>参数：   
>	>
>	>	```
>	>	false : 开启压缩。   
>	>	true : 关闭压缩。   
>	>	```
>
>	* "DisableKeepAlives" 子项
>	>作用: 是否关闭持久连接。   
>	>
>	>参数：   
>	>
>	>	```
>	>	false : 开启持久连接。   
>	>	true : 关闭持久连接。   
>	>	```
>
>	* "IdleConnTimeout" 子项
>	>作用: 闲置连接超时。   
>
>	* "MaxIdleConnsPerHost" 子项
>	>作用: 最大闲置连接数目。   
>
>	* "ResponseHeaderTimeout" 子项
>	>作用: 等待接收服务端的回复的头域的最大时间。   
>
>	* "RetryDelay" 子项
>	>作用: 重试延迟(重试间隔时间)。   
>
>	* "RetryTimes" 子项
>	>作用: 重试次数。   
>

## php.json 配置文件

PHP 代理配置。   

* "Servers" 选项
>这里写上 PHP 代理地址(网址)和密码。可以同时写上多个代理。   
>
>	```
>	"Url": PHP代理服务器地址(网址)
>	"Password": PHP代理密码
>	"SSLVerify": ssl证书验证
>	"Host": PHP代理IP地址(此处也可以填 CDN 域名 或者 SNI proxy IP)
>	```
>
>多个代理格式：   
>
>	```
>	"Servers": [
>		{
>			"Url": "http://yourapp.com/",
>			"Password": "123456",
>			"SSLVerify": true,
>			"Host": "xxx.xxx.xxx.xxx",
>		},
>		{
>			"Url": "https://yourapp.com/",
>			"Password": "123456",
>			"SSLVerify": false,
>			"Host": "",
>		},
>		{
>			"Url": "https://example.com/",
>			"Password": "123456",
>			"SSLVerify": false,
>			"Host": "",
>		}
>	],
>	
>	```

* "Transport" 选项
>作用：传输设置。   
>
>	* "Dialer" 子项
>	>作用：连接配置。   
>	>
>	>	```
>	>	"Timeout": 建立连接超时。
>	>	"KeepAlive": 持久连接保持的时间(时长)。
>	>	"DualStack": 双栈(IPv4和IPv6)开启或者关闭。
>	>	"DNSCacheExpiry": DNS缓存时间(TTL)。
>	>	"DNSCacheSize": DNS缓存大小。
>	>	```
>	>
>
>	* "Proxy" 子项
>	>作用：PHP 代理的前置代理设置。前置代理支持 http、https、ssh、socks4 和 socks5 。   
>	>
>	>	```
>	>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	>	```
>	>	```
>	>	"URL": 前置代理地址。
>	>	
>	>	具体格式如下：
>	>	
>	>	http://XXX.XXX.XXX.XXX:1080         http1代理(http代理)
>	>	http1://XXX.XXX.XXX.XXX:1080         http1代理(http代理)
>	>	https://XXX.XXX.XXX.XXX:1080         https代理
>	>	socks4://XXX.XXX.XXX.XXX:1080       socks4代理
>	>	socks4a://XXX.XXX.XXX.XXX:1080       socks4a代理
>	>	socks://XXX.XXX.XXX.XXX:1080       socks5代理
>	>	socks5://XXX.XXX.XXX.XXX:1080       socks5代理
>	>	ssh://XXX.XXX.XXX.XXX:1080       ssh代理(此项配合 ssh2.json 配置文件使用)
>	>	ssh2://XXX.XXX.XXX.XXX:1080       ssh代理(此项配合 ssh2.json 配置文件使用)
>	>	```
>	>
>	> (r833 httpproxy/proxy/proxy.go#L86-L90)   
>	> (r932 httpproxy/filters/php/php.go#L154-L169)   
>	>
>	>使用前置代理时，关闭以下选项：   
>	>a．关闭配置文件 gae.json 里的 "ForceHTTP2" 选项。   
>	>b．关闭配置文件 gae.json 里的 "EnableDeadProbe" 选项。   
>	>
>
>	* "DisableKeepAlives" 子项
>	>作用：是否关闭持久连接。   
>	>
>	>参数：   
>	>
>	>	```
>	>	false : 开启持久连接。   
>	>	true : 关闭持久连接。   
>	>	```
>
>	* "DisableCompression" 子项
>	>作用：是否关闭压缩。   
>	>
>	>参数：   
>	>
>	>	```
>	>	false : 开启压缩。   
>	>	true : 关闭压缩。   
>	>	```
>
>	* "TLSHandshakeTimeout" 子项
>	>作用：TLS握手超时。   
>
>	* "MaxIdleConnsPerHost" 子项
>	>作用：最大闲置连接数目。   
>

## auth.json 配置文件

认证设置？

* "CacheSize" 选项
>作用：
>

* "Basic" 选项
>作用：
>

* "WhiteList" 选项
>作用：
>

## autoproxy.json 配置文件

自动代理配置。包含 SiteFilters 、 pac 和 gfwlist 等模式。   

* "SiteFilters" 选项
>作用：指定某个域名使用特定代理或者直连。   
>注：要使用 "RegionFilters" 功能，必须开启 httpproxy.json --> Default --> RoundTripFilters --> autoproxy 。   
>
>	```
>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	"Rules": 指定某个域名使用特定代理或者直连(gae/php/direct)。
>	```
>

* "RegionFilters" 选项
>作用：针对目的 IP 归属地选择 gae/php/direct, 可以实现 APN 功能。   
>注：要使用 "RegionFilters" 功能，必须开启 httpproxy.json --> Default --> RequestFilters --> autoproxy 。   
>
>	```
>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	"DataFile": IP 数据库。
>	"DNSCacheSize": DNS 缓存大小。
>	"Rules": 针对目的 IP 归属地选择 gae/php/direct 。
>	```
>

* "IndexFiles" 选项
>作用：自定义 pac 和下载证书等。   
>用浏览器打开地址 http://127.0.0.1:8087/ ，可以看到 "proxy.pac" 和 "GoProxy.crt" 等。具体用途下面介绍。   
>注：要使用 "RegionFilters" 功能，必须开启 httpproxy.json --> Default --> RoundTripFilters --> autoproxy 。   
>
>	* "Enabled" 子项
>	>注意: "Enabled" 子项同时控制 "IndexFiles" 选项和 "GFWList" 选项。   
>	>即，"Enabled" 为 "true"时，GoProxy 会把 proxy.pac 和 gfwlist.txt "整合"到一个 PAC 文件里。   
>	>最终"整合"的 PAC 文件，就是浏览器所使用的 PAC 文件---- http://127.0.0.1:8087/proxy.pac 。   
>	>又，如果 "GFWList" 选项关闭，浏览器所使用的 PAC 文件只有 proxy.pac 。   
>	>
>	>参数:   
>	>	```
>	>	false : 关闭。
>	>	true : 开启。
>	>	```
>	
>	* "Files" 子项
>	>这里分四部份。有"自定义 PAC 文件" "iOS APN 配置文件" "证书下载" "提取格式化导入IP"。   
>	>
>	>	```
>	>	"proxy.pac": 自定义 PAC 文件名。默认没有此文件，第一次使用 PAC 模式时才会自动生成。可自定义此 PAC 文件。
>	>	
>	>	"GoProxyAPN.mobileconfig": iOS APN 配置文件。此项配合 "MobileConfig" 选项使用。
>	>	
>	>	"GoProxy.crt": 证书下载。用浏览器打开地址 http://127.0.0.1:8087/ ，下载此证书，导入到你的操作系统或浏览器。
>	>	
>	>	"ip.html": 提取格式化导入IP。此项配合 "IPHTML" 选项使用。
>	>	```
>	> (httpproxy/filters/autoproxy/autoproxyindex.go)   
>	>
>	>补充：   
>	>证书问题的解决：   
>	>a．Windows 系统，用 "右键" --> "以管理员身份运行" ，会自动导入证书。适用 IE、Chrome 和 Edge 浏览器。   
>	>b．Firefox 浏览器，以上方法不适用，须自行导入证书。   
>	>	>具体步骤如下：   
>	>	>
>	>	>	```
>	>	>	方法一：
>	>	>	1．下载 GoProxy.crt 证书。
>	>	>	
>	>	>	2．打开 FireFox-->选项-->高级-->加密-->查看证书-->证书机构-->导入证书, 选择 GoProxy.crt , 勾选所有项，导入。
>	>	>	```
>	>	>
>	>	>	```
>	>	>	方法二(快捷方法)：
>	>	>	1．用浏览器打开地址 http://127.0.0.1:8087/ ， 单击 "GoProxy.crt" 。
>	>	>	
>	>	>	2．弹出"下载证书"窗口，勾选所有项，按"确定"导入。
>	>	>	```
>	>	>
>	>
>	>c．Linux 系统导入证书。   
>	>	>首次运行，以 root 用户(权限)运行，会自动导入证书。   
>	>	>非本机安装有 GoProxy 情况，以 Debian8 发行版为例，其它发行版可能不适用，请自行 Google 。   
>	>	>注：10.0.1.5 为安装有 GoProxy 电脑的 IP ，并且其开启局域网共享。
>	>	>
>	>	>	```
>	>	>	用浏览器打开地址 http://10.0.1.5:8087/ ，下载 GoProxy.crt 证书。
>	>	>	
>	>	>	sudo mv GoProxy.crt /usr/local/share/ca-certificates/
>	>	>	
>	>	>	sudo update-ca-certificates
>	>	>	```
>	>

* "GFWList" 选项
>作用：gfwlist 黑名单列表。   
>
>	```
>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	"URL": gfwlist.txt 地址(网址)。可自定义。
>	"File": gfwlist 本地文件名。可重命名。
>	"Encoding": 编码。即，gfwlist.txt 使用的编码。
>	"Expiry": 更新间隔时间(更新频率)。即，gfwlist.txt 多久更新一次。默认24小时。
>	"Duration": 重试间隔时间(重试频率)。即，如果下载错误，多久重试一次。默认1小时。
>	```
>

* "MobileConfig" 选项
>作用：移动设备(手机/平板) APN 配置(开关)。   
>此项配合 "GoProxyAPN.mobileconfig" 参数 和 rewrite.json 配置文件里的 "Host" 选项 使用。   
>
>	```
>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	```
>

* "IPHTML" 选项
>作用：导入IP(ip.html 页面文件提取格式化后的IP)到本地 json 文件----控制开关。   
>
>关闭时， http://127.0.0.1:8087/ip.html 页面的 "写入IP到本地 json 文件" 提交按钮无法写法本地 json 文件。   
>
>	```
>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	```
>

* "BlackList" 选项
>作用：网站黑名单列表(或者说去广告)。支持通配符。   
>
>	```
>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	```
>
>	```
>	"SiteRules": 屏蔽网站(网址/IP)列表。
>	```
>

## autorange.json 配置文件

文件(特定范围)自动分段传输(下载)。   

>特定范围：不提供修改，范围有：   
>
>	```
>	"bmp", "gif", "ico", "jpeg", "jpg", "png", "tif", "tiff"   
>	"3gp", "3gpp", "avi", "f4v", "flv", "m4p", "mkv", "mp4"   
>	"mp4v", "mpv4", "rmvb", ".webp", ".js", ".css"   
>	```
>(r833 httpproxy/filters/gae/gaeserver.go#L187)   
>

* "Sites" 选项
>作用：域名筛选。即，自定义哪些网站(网址)使用 autorange 功能。   
>此项配合 "SupportFilters" 选项使用。   

* "SupportFilters" 选项
>作用：支持的代理范围(类型)。范围有："gae","php","vps","direct"。   
>解释：即上述四种代理(或直连)都可以使用 autorange 功能。   
>如果上述某种代理不想使用 autorange 功能可以在其前面注释掉。   
>此项配合 "Sites" 选项使用。   
>

* "MaxSize" 选项
>作用：最大缓存。   

* "BufSize" 选项
>作用：缓存。   

* "Threads" 选项
>作用：线程数。   

## direct.json 配置文件

直连配置。   

* "Transport" 选项
> 作用：传输设置。   
>
>	* "Dialer" 子项
>	>作用：连接配置。
>	>
>	>	```
>	>	"Timeout": 建立连接超时。
>	>	"KeepAlive": 持久连接保持的时间(时长)。
>	>	"DualStack": 双栈(IPv4和IPv6)。
>	>	"DNSCacheExpiry": DNS缓存时间(TTL)。
>	>	"DNSCacheSize": DNS缓存大小。
>	>	```
>	>
>
>	* "Proxy" 子项
>	>作用：direct(直连) 的前置代理设置。前置代理支持 http、https、ssh、socks4 和 socks5 。   
>	>
>	>	```
>	>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	>	```
>	>	```
>	>	"URL": 前置代理地址。
>	>	
>	>	具体格式如下：
>	>	
>	>	http://XXX.XXX.XXX.XXX:1080         http1代理(http代理)
>	>	http1://XXX.XXX.XXX.XXX:1080         http1代理(http代理)
>	>	https://XXX.XXX.XXX.XXX:1080         https代理
>	>	socks4://XXX.XXX.XXX.XXX:1080       socks4代理
>	>	socks4a://XXX.XXX.XXX.XXX:1080       socks4a代理
>	>	socks://XXX.XXX.XXX.XXX:1080       socks5代理
>	>	socks5://XXX.XXX.XXX.XXX:1080       socks5代理
>	>	ssh://XXX.XXX.XXX.XXX:1080       ssh代理(此项配合 ssh2.json 配置文件使用)
>	>	ssh2://XXX.XXX.XXX.XXX:1080       ssh代理(此项配合 ssh2.json 配置文件使用)
>	>	```
>	>
>	> (r833 httpproxy/proxy/proxy.go#L86-L90)   
>	> (r932 httpproxy/filters/direct/direct.go#L110-L132)   
>	>
>	>使用前置代理时，关闭以下选项：   
>	>a．关闭配置文件 gae.json 里的 "ForceHTTP2" 选项。   
>	>b．关闭配置文件 gae.json 里的 "EnableDeadProbe" 选项。   
>	>
>
>	* "TLSClientConfig" 子项
>	>作用：TLS连接-客户端配置。   
>	>
>	>	```
>	>	"InsecureSkipVerify":  是否(跳过)认证服务端的证书链和主机名。
>	>	"ClientSessionCacheSize": TLS会话缓存大小。
>	>	```
>	>
>
>	* "DisableKeepAlives" 子项
>	>作用：是否关闭持久连接。   
>	>
>	>参数：   
>	>
>	>	```
>	>	false : 开启持久连接。   
>	>	true : 关闭持久连接。   
>	>	```
>
>	* "DisableCompression" 子项
>	>作用：是否关闭压缩。(Accept-Encoding/Accept: Gzip)   
>	>
>	>参数：   
>	>
>	>	```
>	>	false : 开启压缩。   
>	>	true : 关闭压缩。   
>	>	```
>
>	* "TLSHandshakeTimeout" 子项
>	>作用：TLS握手超时。   
>
>	* "MaxIdleConnsPerHost" 子项
>	>作用：最大闲置连接数目。   
>

* (无用项--只作格式化用)

## ratelimit.json 配置文件

限速配置。

* "Threshold" 选项
>作用：限制下载带宽(上限)。

* "Rate" 选项
>作用：

* "Capacity" 选项
>作用：

## rewrite.json 配置文件

(http头域)重写配置。   

* "UserAgent" 选项
>作用：http头域User-Agent重写。即，自定义(或伪装)用户代理请求头(UserAgent)。   
>
>	* "Enabled" 子项
>	>作用：选项开关。   
>	>参数：   
>	>
>	>	```
>	>	false : 关闭。   
>	>	true : 开启。   
>	>	```
>	>
>	>注：   
>	>开启 "UserAgent" 的步骤具体如下：   
>	>
>	>	```
>	>	1. httpproxy.json 配置文件 --> "Default" 选项 --> "RequestFilters" 子项，去掉 "rewrite" 前面的双斜杠 "//" 注释符号。
>	>	
>	>	2. 此配置文件里 "Enabled" 子项的参数修改为 "true"。
>	>	```
>
>	* "Value" 子项
>	>用户代理请求头的具体参数值。不懂或者不理解 http 协议的不要修改。   
>	>
>

* "Host" 选项
>作用：http头域Host重写。   
>
>	```
>	"Enabled": 选项开关。参数: true(开启) 或者 false(关闭)。
>	
>	"RewriteBy": 重写的具体字段(或者值)。"X-Online-Host" 这个主要用于移动平台(wap方式)，是一个私有协议字段。
>	```
>
>补充：   
>"X-Online-Host" 字段 配合 autoproxy.json 配置文件里的 "MobileConfig" 选项 使用。
>

## stripssl.json 配置文件

伪造与替换 https/ssl 证书。   

* "RootCA" 选项
>作用：SSL根证书生成设置。   
>
>	```
>	"Name": (GoProxy 生成的) SSL根证书名。
>	"Dirname": (GoProxy 生成的) SSL证书缓存目录。
>	"Duration": (GoProxy 生成的) SSL根证书有效时间。默认一年。
>	"RSABits": SSL证书密钥长度。默认2048位。
>	"Portable": 缓存目录位置("Dirname")控制。(判断是否是在 GoProxy 所在目录 )
>	```
>

* "Ports" 选项
>作用：ssl strip (尝试的)特定端口。(tlsConn.Handshake)   

* "Sites" 选项
>作用：域名筛选。   

## ssh2.json 配置文件

ssh2 前置代理配置。   

* "servers" 选项
>服务器端设置。   
>
>	```
>	"Addr": ssh2代理地址。
>	"Username": 用户名。
>	"Password": 密码。
>	```
>

* "Transport" 选项
> 作用：传输设置。   
>
>	```
>	"DisableKeepAlives": 是否关闭持久连接。
>	"DisableCompression": 是否关闭压缩。
>	"TLSHandshakeTimeout": TLS握手超时。   
>	"MaxIdleConnsPerHost": 最大闲置连接数目。   
>	```
>

## vps.json 配置文件

VPS 代理配置。   

* "servers" 选项
>服务器端设置。   
>
>	```
>	"Url": VPS代理地址(网址)。
>	"Username": 用户名。
>	"Password": 密码。
>	"SSLVerify": ssl证书验证。
>	```
>

* (无用项--只作格式化用)

## addto-startup.vbs 脚本文件

>用途：设置 GoProxy 开机启动。(Windows系统)   

## get-latest-goproxy.cmd 批处理文件

>用途：升级到最新版。(Windows系统)   

## ip.html 页面文件

>用途：提取并格式化IP。   
>
>也可以直接在浏览器中输入 http://127.0.0.1:8087/ip.html 打开。   
>
>例子：   
>
>扫到IP结果如下：   
>
>	```
>	61.238.239.240 558 *.googlevideo.com gvs 1.0
>	115.164.12.156 559 *.googlevideo.com gvs 1.0
>	113.171.242.226 561 google.com gws
>	```
>
>提取并格式化IP后结果如下：   
>
>	```
>	"61.238.239.240",
>	"115.164.12.156",
>	"113.171.242.226",
>	```
>

## gae.user.json.example 文件

>用途：gae.user.json 样本文件。
>