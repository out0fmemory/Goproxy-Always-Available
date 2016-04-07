配置介绍

## 前言   

　　以当前r345正式版为准。以后版本有出入再修改。这是新人入门用的。不是最详细的配置介绍。以能运行能用
为准。当前版本下，最主要的配置文件为gae.json，经常修改的都是这个。还有一个是main.json。下面主要介绍
这两个文件最常用的地方。   
　　注：  
1．当前版本已支持autorange。   
2．当前版本格式已较宽松，即末尾有无逗号都可以。   
3．当前版本支持 xxx.user.json 命名文件----即，如 gae.json，复制 gae.json 为 gae.user.json 并修改保存。下次新版更新直接覆盖 gae.json 没问题。  
4. 善用搜索。礼貌提问。提问之前最好搜索一下。   
题外话：这是自由软件(Free Software)，没有人有问答你问题的义务。别人都很忙，别人时间都是宝贵的。

## gae.json 文件

* "AppIDs" 选项

　　这个是加入你的 Google Appengine 的帐号。格式如下：  

	```
    "AppIDs": [  
        "ID1" ,   
        "ID2" ,    
        "ID3"  
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
    
* "HostMap" 选项

这里填写你找到的IP。格式如下：

	```
	"HostMap" : {
		"google_hk": [
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx"
		],
		"google_talk": [
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx"
		],
		"google_cn": [
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx",
            "xxx.xxx.xxx.xxx"
		]
	},
	```
    
## main.json 文件

* "Addr" 选项

这里一般不用改。格式： "IP地址:端口"

这里常用有两种(任选一种)：

	```
	"Addr": "127.0.0.1:8087",
    
	"Addr": "0.0.0.0:8087",
	```
    
注：   
1. 第一种是只能本机使用。   
2. 第二种是用来共享时使用。当然本机也能用。

* "Filters" 选项

说明：双斜杠 "//" 是注释符号，取消即使用。   
以下是开启autorange，并且使用gae时的实例：

	```
	"Filters": {
		"Request": [
			// "auth",
			"stripssl",
			"autorange",
		],
		"RoundTrip": [
			"autoproxy",
			// "auth",
			// "vps",
			//"php",
			"gae",
			"direct",
		],
		"Response": [
			"autorange",
			// "ratelimit",
		]
	}
	```