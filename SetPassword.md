默认情况下，只要知道你的appid就可以使用你的流量，加上密码之后，他人即使知道你的appid也没法使用你的流量。
### GAE

用文本编辑器打开 server/gae 文件夹下的 gae.py ,不建议使用 Windows 自带的记事本，推荐 [notepad++](http://notepad-plus-plus.org/) 或者 [notepad2](http://www.flos-freeware.ch/notepad2.html)
- 在开始部分找到

    ```
    __password__ = '123456'
    ```

- 把单引号中的`123456`换成你的密码如：12345678

    ```
    __password__ = '12345678'
    ```

保存之后，使用uploader.bat重新上传，密码即可生效。
- 自己使用时只需在proxy.ini填上密码即可；如果有多个appid，只能设置相同的密码。

    ```
    [gae]
    appid = appid1|appid2
    password = 12345678
    ```

- 要取消密码，清空gae.py中密码之后，再重新上传即可，每次服务端升级时需要重新设置密码。


### PHP
默认密码为 123456，且必须设置密码

- python版，请在server/php/index.py，将 123456 替换成你自己的密码

    ``` 
    __password__ = '123456'
    ```

- PHP版，打开server/php/index.php，将 123456 替换成你自己的密码

    ```
    $__password__ = '123456';
    ```

- nodejs版，打开server/php/index.js，将 123456 替换成你自己的密码

    ```
    var __password__ = '123456';
    ```
- 自己使用时将密码填至proxy.ini中

    ```
    [php]
    enable = 1
    password = 12345678
    ```

## hostsdeny说明及用法

该功能是在你将自己的appid给他人使用，又不想别人使用你的appid访问某些网站，可以将禁止访问的网站加如hostsdeny中。
- 在gae.py中，开头部分有
    
    ```
    __hostsdeny__ = ()
    ```

- 假如要禁止youtube、土豆和优酷的访问，就可以如下设置

    ```
    __hostsdeny__ = ('.youtube.com', '.youku.com', '.tudou.com')
    ```

- 注意：符号为英文半角符号

## relay.php说明及用法

relay.php是用来为PAAS转发的。当你已经在国外部署了PAAS版，但是自己直接连接国外很慢，而你又有国内服务器，服务器连接国外的速度很快，在国内服务器上部署relay.php，你→relay.php→国外PAAS→目标网站，从而达到加速的目的。
    
    //PAAS版的地址
    $__relay__ = 'http://goagent.app.com/index.php';
    //用来连接的地址列表，一般使用PAAS版的域名即可
    $__hosts__ = array('goagent.app.com');   

如果有多个地址都可以解析到PAAS的地址，比如`1.1.1.1`和`2.2.2.2`都可以解析到我的PAAS，则可以设置如下。注意：有些服务器可能不支持这种设置，一般请按上面设置即可。
    
    $__hosts__ = array('1.1.1.1', '2.2.2.2');

