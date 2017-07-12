## 常见问题

1. goproxy 和 goagent 的关系？
> 我（phuslu) 对 goagent 对 goproxy 的重写，设计理念和使用习惯是一脉相承的。

2. 需要重新上传 gae 服务端吗?
> 需要, goproxy 的服务端更高效更抗干扰，老的 goagent 服务端和客户端不兼容。

3. 源代码在哪里，怎么编译？
> 代码：https://github.com/phuslu/goproxy/tree/master
> 编译：https://github.com/phuslu/goproxy/blob/wiki/HowToBuild.md

4. goproxy 原理是什么？
> 原理可以参考这个图 ![代理示意图](https://cloud.githubusercontent.com/assets/195836/4602738/ac950aba-5149-11e4-8976-a2606ba08e05.png)

5. 安全性如何 ？
> goproxy 使用中间人攻击来代理 HTTPS 请求，根证书在第一次运行生成。安全性可以类比 fiddler/charles.
> gae.json 的 SSLVerify 选项打开之后，可以抵御网关签署假的谷歌证书来解析你的流量。
