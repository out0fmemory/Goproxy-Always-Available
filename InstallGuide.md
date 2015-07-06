#图文教程

##一、申请 Google App Engine 并创建 appid
  1. 申请注册一个 [Google App Engine](https://appengine.google.com) 账号。没有 Gmail 账号先注册一个， 用你的 Gmail 账号登录。  
    ![申请 GAE 帐号](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e4thu7y0pgj20hs0atgn9.jpg)
  1. 登录之后，自动转向 Application 注册页面，如下图：  
    ![申请 GAE 帐号](http://i.imgur.com/kDCyFnL.png)
    点击“Create Application”
    ![申请 GAE 帐号](http://i.imgur.com/M8wXgtH.png)
    点击“Google Developers Console”
    ![申请 GAE 帐号](http://i.imgur.com/bxuKVUC.png)
    点击“Create a project”
    ![申请 GAE 帐号](http://i.imgur.com/fdUGhKw.png)
    填写"Project name"，大小写、数字随意，但注意“Project ID”（即原来的appid）全部是小写
    新版Google App Engine可以选择US或EU data center（美国或欧洲数据中心）
    勾选同意协议，点击“Create”开始创建Project，约1-2分钟后，看到如下图说明创建完成
    ![申请 GAE 帐号](http://i.imgur.com/dHcWO18.png)
    点击“Create a project”可重复创建，
    ![申请 GAE 帐号](http://i.imgur.com/u26RJzv.png)
    - 一个Gmail账户最多可以创建25个GAE应用，每个应用每天 1G 免费流量。这里我们只创建一个应用就可以了。

##二、下载 goagent 并上传至 Google App Engine
  1. 下载 goagent 并解压，<https://github.com/goagent/goagent>
  1. 编辑 local\proxy.ini，把其中 appid = goagent 中的 goagent 改成你之前申请的应用的 appid (用 windows 的记事本也可以)
    - 如果要使用多个appid，appid之间用|隔开，如：appid1|appid2|appid3，每个 appid 必须确认上传成功才能使用

    ```
    [gae]
    appid = appid1|appid2|appid3
    ```

  1. 运行 goagent.exe(Linux 用python运行XXX/local文件下的goagent-gtk.py来启动goagent)
  1. 上传
    - Windows 用户：双击 server 文件夹下的 uploader.bat，输入你上步创建的 appid (同时上传多 appid 在 appid 之间用 | 隔开,一次只能上传同一个谷歌帐户下的 appid )填完按回车。根据提示填你的谷歌帐户邮箱地址，填完按回车。根据提示填你的谷歌帐户密码(注意：如果开启了两步验证，密码应为[16位的应用程序专用密码](https://accounts.google.com/b/0/IssuedAuthSubTokens)而非谷歌帐户密码，否则会出现 AttributeError: can't set attribute 错误)，填完按回车。
    - Linux/Mac OSX用户上传方法：在server目录下执行：python uploader.py  
    ![上传 GAE](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw9e7n5u3iijqj20iq0c8q6b.jpg)
    - 如遇到上传困难的情况，可以先运行 goagent.exe(要先修改 appid )再运行 uploader.bat
    - 上传成功就会看图下图界面  
    ![上传 GAE](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e3bnmhap9wj.jpg_uploaded.png)


##三、运行客户端
  1. Windows 用户运行 local 文件夹中的 goagent.exe，Linux/MacOSX 用户运行 python proxy.py
    - 设置浏览器或其他需要代理的程序代理地址为 127.0.0.1:8087
    - 注意：使用过程中要一直运行 goagent.exe/proxy.py
    - 代理地址 127.0.0.1:8087；如需使用 PAC，设置pac地址为 <http://127.0.0.1:8086/proxy.pac>
    - 也可以配合 SwitchyOmega/FoxyProxy 等浏览器扩展 (SwitchyOmega 用户可从 local 文件夹中的 SwitchyOptions.bak 文件导入配置)
  1. 导入证书
    - IE/Chrome：使用管理员身份运行 goagent.exe 会自动向系统导入 IE/Chrome 的证书，你也可以双击 local 文件夹中的 CA.crt 安装证书(需要安装到 "受信任的根证书颁发机构" )；  
    ![运行客户端](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e6mc176ngnj20bn0dit9l.jpg)  
    ![运行客户端](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e6mc184ah3j20e00e6wg1.jpg)
    - 下一步 -> 完成 -> 确定
    - Firefox：需要单独导入证书，打开FireFox?->选项->高级->加密->查看证书->证书机构->导入证书, 选择 local\CA.crt, 勾选所有项，导入；
    - Opera：导入证书方法：首选项→高级→安全性→管理证书→证书颁发机构->导入->选择 local\CA.crt文件->依次确认

##浏览器设置方法
  1. 使用 GoAgent 自带代理设置功能
    - 该功能可以为 IE 内核浏览器和未安装代理类扩展的 Chrome/Opera 等默认使用 IE 代理的浏览器和软件设置代理，但不能给 FireFox 设置代理
    - 右击 GoAgent 托盘图标，在 "设置IE代理" 菜单中选择要使用的模式。
    - 禁用代理 什么也不做，需要用户自己手动为软件设置代理
    - <http://127.0.0.1:8086/proxy.pac> 使用自带的PAC自动判断是否使用代理
    - <127.0.0.1:8087> 全部使用 GoAgent 代理
  1. 谷歌 Chrome 配合 SwitchyOmega 扩展
    - 安装扩展
    - 下载 [SwitchyOmega.crx](https://github.com/FelisCatus/SwitchyOmega/releases), 地址栏输入 Chrome://extensions/ 后按回车，打开扩展管理页，拖拽 SwitchyOmega.crx 到该页面之后点击确定即可安装，扩展也可以从 [Chrome 应用商店](https://chrome.google.com/webstore/detail/proxy-switchyomega/padekgcemlokbadohgkifijomclgjgif) 获得  
    ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e3hhmzjy1zj.jpg_install_Proxy_Switchy_Sharp.png)
    - 导入设置
      - 点击 SwitchyOmega 图标->导入/导出  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e2s44kpzqyj.jpg_bak.png)
      - 浏览到 SwitchyOptions.bak，点击确定导入设置
      - 更新自动切换规则(如果遇到无法更新规则列表，可以先运行 goagent ，并把浏览器代理设置为 GoAgent 模式再更新规则，不更新规则只会影响自动切换模式，不会影响其他模式的使用，若确实无法更新也可不更新，直接使用 PAC 模式即可)
      - 在扩展设置页点击 "自动切换" ，点击 "立即更新情景模式" ，最后点击 "应用选项" 。  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3tcf8lij.jpg_getrules.png)
    - 单击地址栏右侧 Proxy SwitchyOmega 图标即可进行模式选择  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3t6x2ivj.jpg_changemode.png)
      - GoAgent 模式 除匹配 proxy.ini 中 profile 的直连外，其他全部通过 GAE
      - GoAgent PAAS 模式 全部通过 PAAS
      - GoAgent PAC 模式 根据 GoAgent 自带的 PAC 文件自动判断是否经过代理
      - 自动切换模式 根据切换规则自动选择是否进行代理，并根据所设情景模式自动选择使用何种代理
      - 遇到规则中没有的，可以使用扩展的 "新建规则" 按钮自行添加，选情景模式为 "GoAgent" ，使用此模式可以方便的定制自己的代理切换规则
      - 如果设置无误但浏览器提示错误 130 无法连接到代理服务器，可以将自己的设置导出之后卸载重装
      - 如果遇到无法更新规则列表，可以先运行 goagent，并把浏览器代理设置为 GoAgent 模式再更新规则，不更新规则只会影响自动切换模式，不会影响其他模式的使用，若确实无法更新也可不更新，直接把扩展设置为 GoAgent PAC 模式即可
  1. Firefox 配合 FoxyProxy 扩展
    - 安装扩展 <https://addons.mozilla.org/zh-cn/firefox/addon/foxyproxy-standard/>
    - 设置  
    ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3t8whfdj.jpg_foxyproxy.png)
      - 右击 foxyporxy 图标即可选择代理模式  
    ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3taih9wj.jpg_foxyproxy1.png)
    - 添加代理规则订阅(可选)
      - 这里以添加 [gfwlist](http://autoproxy-gfwlist.googlecode.com/svn/trunk/gfwlist.txt) 为例，你也可以自行添加其他规则订阅  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e3f79aksi6j.jpg)  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e3f7955znpj.jpg)  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e3f797nabpj.jpg)  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e3f79bigcuj.jpg)
    - 更多设置请自行探究
  1. Firefox 配合 AutoProxy 扩展(新版Firefox请将此扩展升级至最新版)
    - 安装扩展 <https://addons.mozilla.org/zh-cn/firefox/addon/autoproxy/>
    - 设置
      - 添加代理服务器 注意:新版 autoproxy 已内置GoAgent配置，可直接进行下一步  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3t49g0ej.jpg_autoproxyfirst.png)  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3t08ft0j.jpg_autoproxy1.png)  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3t0sibvj.jpg_autoproxy2.png)
      - 添加规则订阅  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3t241zej.jpg_autoproxyaddrules1.png)  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3t380m4j.jpg_autoproxyaddrules2.png)
      - 选择自己需要的模式  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3szej8sj.jpg_autoproxy.png)
        - 自动模式   根据规则自行选择是否使用代理
        - 全局模式   全部使用代理
        - 禁用代理   全部不使用代理
  1. opera 浏览器设置
    - 同 IE 一样有两种方式可选，不过不会影响系统其他程序的联网
    - 设置代理为 127.0.0.1:8087，全部使用 goagent 代理  
    ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3tl5ww7j.jpg_opera1.png)
    - 使用 PAC 自动代理  
    ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887tw1e2s3tlvyqmj.jpg_opera-pac.png)
    - 不使用时应恢复为无代理状态
  1. IE浏览器设置
    - 工具->Internet 选项->连接，局域网用户单击"局域网设置"。宽带用户选中自己正在使用的宽带连接之后单击"设置"，不要选 "局域网设置"
    - 局域网用户设置方法
      - 设置代理为 127.0.0.1:8087 ，全部使用 goagent 代理(不建议)  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e3ewkxcosfj.jpg_ie1.png)
      - 使用 PAC 自动代理  
      ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e3ewkyd12nj.jpg_ie2.png)
    - 宽带用户设置方法
      - 选中自己正在使用的宽带连接之后单击"设置"  
        ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e6fvndhycoj20al04zmxg.jpg)
      - 设置代理为 127.0.0.1:8087，全部使用 goagent 代理(不建议)  
        ![浏览器设置](https://raw.githubusercontent.com/goagent/goagent/wiki/InstallGuideImages/786e2887jw1e6fvnd4bicj20c5088my2.jpg)
   - 不使用时要将IE恢复无代理状态


##适用环境
  - 适用：浏览器，支持 http 代理的下载软件等
  - 不适用：游戏客户端等需要稳定网络的程序，QQ，Tor(验证证书)。待添加。。。

##关于软件更新
  - 更新历史中带有[是]则需要重新上传，否则不用重新上传。注意：是否需要重新上传是相对于前一版的，若你之前版本与当前版本之间某一版或多版带有[是]仍然需要重新上传。
  - appid 并不绑定任何客户端，如果本次更新无需重新上传，只需修改 proxy.ini 中的 appid 即可使用。同样，你也可以把 appid 共享给朋友，或者在自己其他机器上使用，一个 appid 可以多人多机器同时使用，在无需更新服务端的情况下，只需成功上传一次即可。在没有设定密码的情况下，只需要知道 appid 就可以使用你的 appid 的流量，为防止被盗用可以[加上密码](https://github.com/goagent/goagent/blob/wiki/SetPassword.md)。
  - goagent 每一版下载的都是全部文件，你可以选择覆盖原文件或者将新版放另一个文件夹，旧版你可以选择留存或者删除，修改新版 proxy.ini 中相关设置即可运行。如果旧版添加了开机启动，需要将旧开机启动删除。如果旧版已经在运行，需先将旧版关闭。
  - 如果之前版本没有ssl错误，使用新版出现 ssl 错误可以把原来的 CA.crt 和 certs 文件夹内的文件覆盖当前的这些文件。或者将 CA.crt 和 certs 文件夹删除，同时删除浏览器中所有goagent ca 的证书，再重启 goagent，会生成新证书，重启浏览器再导入新证书即可。浏览器证书中只能有一个 goagent ca 的证书。
