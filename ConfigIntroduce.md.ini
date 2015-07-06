# 配置介绍

    [listen]
    #监听ip，如果需要允许局域网/公网使用，设为0.0.0.0即可
    ip = 127.0.0.1
    #使用GAE服务端的默认8087端口，如有需要你可以修改成其他的
    port = 8087
    #8087端口验证的用户名和密码，设置后可以防止局域网内其他人使用你的GoAgent
    username =
    password =
    #启动后goagent窗口是否可见，0为不可见（最小化至托盘），1为不最小化
    visible = 1
    #是否显示详细debug信息
    debuginfo = 0
    
    #GAE 服务端的配置
    [gae]
    #是否启用 GAE 服务端。
    enable = 1
    #你的 Google Appengine AppID, 也就是服务器部署的 APPID，配置多 ID 用|隔开
    appid = goagent
    #密码,默认为空,你可以在 server 目录的 gae.py 设定,如果设定了,此处需要与 gae.py 保持一致
    password = 123456
    #服务端路径,一般不用修改,如果不懂也不要修改.
    path = /_gh/
    #使用http还是 https(SSL 加密传输)连接至GAE
    mode = https
    #是否启用 ipv6
    ipv6 = 0
    #ip评优算法每次选出的 i p数量
    window = 6
    #是否缓存ip评优算法生成的临时连接
    cachesock = 1
    #连接 ip 后是否使用 http HEAD 请求测试, 启用可以更好的测试该 ip 的质量。
    headfirst = 1
    #是否使用 http/1.1 的 keepalive 功能
    keepalive = 0
    #是否开启流量混淆
    obfuscate = 0
    #是否通过 pagespeed 服务中转访问 GAE
    pagespeed = 0
    #是否对服务器证书进行验证
    validate = 0
    #是否打开透明代理功能（和iptables配合使用）
    transport = 0
    # 如果设置为 rc4 则开启 rc4 加密，需在 password 设置密码，否则不开启，一般mode为https时无需开启
    options =
    #根据IP所在地区设置是否直连，比如 regions = cn|jp 可以让国内和日本的网站走直连。更多国家请见 <http://dev.maxmind.com/geoip/legacy/codes/iso3166/>
    regions =
    #每次urlfetch最大返回的文件大小
    maxsize = 2097152
    
    # 用于连接GAE的IP列表
    [iplist]
    google_cn = www.g.cn|www.google.cn
    google_hk = www.google.com|mail.google.com|www.google.com.hk|www.google.com.tw|www.l.google.com|mail.l.google.com
    google_talk = talk.google.com|talk.l.google.com|talkx.l.google.com
    google_ipv6 = ipv6.google.com
    
    # 匹配规则，支持 host 配置， host 后缀匹配，和 url 正则匹配
    # 匹配规则有： 1. withgae 优先走 gae
    #              2. withphp 优先走 php 
    #              3. direct 直连
    #              4. fakehttps 使用 goagent 证书替换网站本身证书
    #              5. nofakehttps 禁用 goagent 证书替换网站本身证书
    #              6. forcehttps 强制 http 连接跳转到 https 网址
    #              7. noforcehttps 禁用 http 连接跳转到 https 网址
    #              8. google_* 使用 iplist 提供的地址直连
    [profile]
    play.google.com = withgae
    wenda.google.com.hk = withgae
    clients.google.com = withgae
    scholar.google.com = nocrlf,noforcehttps,nofakehttps
    scholar.google.com.hk = nocrlf,noforcehttps,nofakehttps
    scholar.google.com.cn = nocrlf,noforcehttps,nofakehttps
    books.google.com.hk = nocrlf,noforcehttps,nofakehttps
    webcache.googleusercontent.com = crlf,noforcehttps,nofakehttps
    mtalk.google.com = direct
    talk.google.com = google_talk
    talk.l.google.com = google_talk
    talkx.l.google.com = google_talk
    1-ps.googleusercontent.com = google_cn
    2-ps.googleusercontent.com = google_cn
    3-ps.googleusercontent.com = google_cn
    4-ps.googleusercontent.com = google_cn
    .google.cn = google_cn
    .appspot.com = google_hk,crlf
    .google.com = google_hk,forcehttps,fakehttps
    .google.com.hk = google_hk,forcehttps,fakehttps
    .googleapis.com = google_hk,forcehttps,fakehttps
    .googleusercontent.com = google_hk,forcehttps,fakehttps
    .googletagservices.com = google_hk,forcehttps,fakehttps
    .googletagmanager.com = google_hk,forcehttps,fakehttps
    .google-analytics.com = google_cn,forcehttps,fakehttps
    .gstatic.com = google_hk,fakehttps
    .ggpht.com = google_hk,fakehttps
    .googlegroups.com = google_hk,forcehttps,fakehttps
    .googlecode.com = google_hk,forcehttps,fakehttps
    .youtube.com = forcehttps,fakehttps
    .android.com = google_hk
    www.dropbox.com = withgae
    .dropbox.com:443 = direct
    .box.com:443 = direct
    .copy.com:443 = direct
    https?://www\.google\.com/(?:imgres|url)\?.*url=([^&]+) = $1
    https?://www\.google\.com\.hk/(?:imgres|url)\?.*url=([^&]+) = $1
    #针对指定URL返回一个本地文件
    ; https?://www\.example\.com/.+\.html = file:///C:/README.txt
    #取消注释（删除行首分号）使用 google_cn 地址作为 google 搜索
    ; https?://www\.google\.com(\.[a-z]{2})?/($|(search|url|gen_204)\?|(complete|images)/) = google_cn
    #取消注释（删除行首分号）播放youtube上地区限制的vevo视频，此法可正常播放大部分vevo视频
    ; https?://www\.youtube\.com/watch = google_hk
    #取消注释看直播
    ; .c.youtube.com =
    ; .youtube.com = google_hk
    ; .googlevideo.com =
    
    
    #代理自动配置脚本(Proxy auto-config)设定
    [pac]
    #是否启用，若启用，浏览器代理自动配置地址填http://127.0.0.1:8086/proxy.pac
    enable = 1
    # pacserver的监听地址
    ip = 127.0.0.1
    port = 8086
    # pac文件的名称
    file = proxy.pac
    #被墙规则订阅地址
    gfwlist = http://autoproxy-gfwlist.googlecode.com/svn/trunk/gfwlist.txt
    #广告拦截规则订阅地址
    adblock = http://adblock-chinalist.googlecode.com/svn/trunk/adblock.txt
    #自动更新间隔时间
    expired = 86400
    
    #对应php server 的设置
    [php]
    enable = 0
    password = 123456
    crlf = 0
    validate = 0
    listen = 127.0.0.1:8089
    fetchserver = https://.cm/
    
    #二级代理,一般内网会用到
    [proxy]
    #是否启用
    enable = 0
    autodetect = 1
    #代理服务器地址
    host = 10.64.1.63
    #代理服务器端口
    port = 8080
    #代理服务器登录用户名
    username = username
    #密码
    password = 123456
    
    # 自动分段下载，需远程服务器支持Rang
    [autorange]
    #匹配以下域名时自动下载
    hosts = *.c.youtube.com|*.atm.youku.com|*.googlevideo.com|*av.vimeo.com|smile-*.nicovideo.jp|video.*.fbcdn.net|s*.last.fm|x*.last.fm|*.x.xvideos.com|*.edgecastcdn.net|*.d.rncdn3.com|cdn*.public.tube8.com|videos.flv*.redtubefiles.com|cdn*.public.extremetube.phncdn.com|cdn*.video.pornhub.phncdn.com|*.mms.vlog.xuite.net|vs*.thisav.com|archive.rthk.hk|video*.modimovie.com|*.c.docs.google.com
    # 自动对列表中文件类型启用分段下载功能
    endswith = .f4v|.flv|.hlv|.m4v|.mp4|.mp3|.ogg|.avi|.exe|.zip|.iso|.rar|.bz2|.xz|.dmg
    # 禁用分段下载的文件类型
    noendswith = .xml|.json|.html|.php|.py|.js|.css|.jpg|.jpeg|.png|.gif|.ico|.webp
    # 线程数
    threads = 3
    #一次最大下载量
    maxsize = 1048576
    #首次读写量
    waitsize = 524288
    #后续读写量
    bufsize = 8192
    
    #DNS模块，可以用来防止DNS劫持/污染
    [dns]
    enable = 0
    #DNS监听地址，使用时将系统DNS设置为127.0.0.1
    listen = 127.0.0.1:53
    #远程DNS查询服务器
    remote = 8.8.8.8|8.8.4.4|114.114.114.114|114.114.115.115
    #缓存大小
    cachesize = 5000
    #超时时间
    timeout = 2
    
    #模拟用户浏览器类型,在User-Agent里提交给服务器你的浏览器操作系统等信息
    [useragent]
    #是否启用
    enable = 0
    #可自行修改的，前提是你知道怎么改
    string = Mozilla/5.0 (iPhone; U; CPU like Mac OS X; en) AppleWebKit/420+ (KHTML, like Gecko) Version/3.0 Mobile/1A543a Safari/419.3
    
    [fetchmax]
    local =
    server =
    
    #不用理会,显示在控制台上方的公益广告
    [love]
    #不愿意看到这广告就把1改成0
    enable = 1
    tip = \u8bf7\u5173\u6ce8\u5317\u4eac\u5931\u5b66\u513f\u7ae5~~


